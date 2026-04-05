package exporter

import (
	"bufio"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ExportProgress struct {
	TableName         string    `json:"table_name"`
	LastExportedValue string    `json:"last_exported_value"`
	RowsExported      int64     `json:"rows_exported"`
	TotalRows         int64     `json:"total_rows"`
	StartTime         time.Time `json:"start_time"`
	LastUpdate        time.Time `json:"last_update"`
	Status            string    `json:"status"`
	FileName          string    `json:"file_name"`
}

type ResumableExporter struct {
	db          *sql.DB
	config      *ExporterConfig
	progressDir string
}

func NewResumableExporter(db *sql.DB, config *ExporterConfig, progressDir string) *ResumableExporter {
	os.MkdirAll(progressDir, 0755)
	return &ResumableExporter{db: db, config: config, progressDir: progressDir}
}

func (re *ResumableExporter) ExportWithResume(tableName string) error {
	progress := re.loadProgress(tableName)

	if progress != nil && progress.Status == "running" {
		log.Printf("Resuming %s from %s", tableName, progress.LastExportedValue)
		return re.exportInBatches(progress)
	}

	return re.startNewExport(tableName)
}

func (re *ResumableExporter) startNewExport(tableName string) error {
	var totalRows int64

	err := re.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)).Scan(&totalRows)
	if err != nil {
		return err
	}

	if totalRows == 0 {
		log.Printf("Table %s empty", tableName)
		return nil
	}

	filename := fmt.Sprintf("data_%s_resumable.sql", tableName)

	progress := &ExportProgress{
		TableName: tableName,
		TotalRows: totalRows,
		StartTime: time.Now(),
		Status:    "running",
		FileName:  filename,
	}

	re.saveProgress(progress)

	return re.exportInBatches(progress)
}

func (re *ResumableExporter) exportInBatches(progress *ExportProgress) error {

	columns, columnTypes, err := re.getTableColumns(progress.TableName)
	if err != nil {
		return err
	}

	// 🔥 FIXED PK DETECTION
	pkColumn, pkIndex, err := re.getPrimaryKey(progress.TableName, columns)

	// 🔥 FALLBACK IF NO PK
	if err != nil || pkColumn == "" {
		log.Printf("⚠️ Table %s has no primary key → fallback export", progress.TableName)
		return re.exportWithoutPK(progress, columns, columnTypes)
	}

	filePath := filepath.Join(re.config.OutputDir, progress.FileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024)
	defer writer.Flush()

	if progress.RowsExported == 0 {
		writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", progress.TableName))
		writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", progress.TableName))
	}

	batchSize := re.config.RowsPerBatch
	bulkSize := re.config.BulkInsertSize
	if bulkSize <= 0 {
		bulkSize = 1000
	}

	var buffer []string
	startTime := time.Now()
	lastSave := time.Now()

	for {

		query := fmt.Sprintf(`
			SELECT * FROM `+"`%s`"+`
			WHERE `+"`%s`"+` > ?
			ORDER BY `+"`%s`"+`
			LIMIT ?
		`, progress.TableName, pkColumn, pkColumn)

		rows, err := re.db.Query(query, progress.LastExportedValue, batchSize)
		if err != nil {
			progress.Status = "failed"
			re.saveProgress(progress)
			return err
		}

		count := 0
		var batchLastVal string

		for rows.Next() {

			values := make([]interface{}, len(columns))
			ptrs := make([]interface{}, len(columns))
			for i := range values {
				ptrs[i] = &values[i]
			}

			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return err
			}

			// SAFE PK extraction
			val := values[pkIndex]
			switch v := val.(type) {
			case int64:
				batchLastVal = fmt.Sprintf("%d", v)
			case int32:
				batchLastVal = fmt.Sprintf("%d", v)
			case int:
				batchLastVal = fmt.Sprintf("%d", v)
			case []byte:
				batchLastVal = string(v)
			case string:
				batchLastVal = v
			default:
				batchLastVal = fmt.Sprintf("%v", v)
			}

			buffer = append(buffer, re.buildValuesOnly(values, columnTypes))

			if len(buffer) >= bulkSize {
				re.writeBulkInsert(writer, progress.TableName, columns, buffer)
				buffer = buffer[:0]
			}

			count++
		}

		rows.Close()

		if count == 0 {
			break
		}

		// flush remaining
		if len(buffer) > 0 {
			re.writeBulkInsert(writer, progress.TableName, columns, buffer)
			buffer = buffer[:0]
		}

		// 🔥 DUPLICATE FIX
		if batchLastVal == progress.LastExportedValue {
			log.Printf("Duplicate cursor detected → stopping %s", progress.TableName)
			break
		}

		progress.LastExportedValue = batchLastVal
		progress.RowsExported += int64(count)
		progress.LastUpdate = time.Now()

		if progress.RowsExported%100000 == 0 || time.Since(lastSave) > 30*time.Second {
			re.saveProgress(progress)
			lastSave = time.Now()

			elapsed := time.Since(startTime)
			rate := float64(progress.RowsExported) / elapsed.Seconds()
			remaining := float64(progress.TotalRows - progress.RowsExported)
			eta := time.Duration(remaining/rate) * time.Second

			log.Printf("%s: %d/%d (%.1f%%) %.0f rows/sec ETA %v",
				progress.TableName,
				progress.RowsExported,
				progress.TotalRows,
				float64(progress.RowsExported)/float64(progress.TotalRows)*100,
				rate,
				eta)
		}

		if progress.RowsExported%10000 == 0 {
			writer.Flush()
		}
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", progress.TableName))
	writer.WriteString("UNLOCK TABLES;\n")

	progress.Status = "completed"
	re.saveProgress(progress)

	return nil
}

//
// ---------- FALLBACK (NO PK) ----------
//

func (re *ResumableExporter) exportWithoutPK(
	progress *ExportProgress,
	columns []string,
	columnTypes []*sql.ColumnType,
) error {

	filePath := filepath.Join(re.config.OutputDir, progress.FileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024)
	defer writer.Flush()

	writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", progress.TableName))
	writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", progress.TableName))

	query := fmt.Sprintf("SELECT * FROM `%s`", progress.TableName)

	rows, err := re.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	bulkSize := re.config.BulkInsertSize
	if bulkSize <= 0 {
		bulkSize = 1000
	}

	var buffer []string

	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return err
		}

		buffer = append(buffer, re.buildValuesOnly(values, columnTypes))

		if len(buffer) >= bulkSize {
			re.writeBulkInsert(writer, progress.TableName, columns, buffer)
			buffer = buffer[:0]
		}
	}

	if len(buffer) > 0 {
		re.writeBulkInsert(writer, progress.TableName, columns, buffer)
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", progress.TableName))
	writer.WriteString("UNLOCK TABLES;\n")

	log.Printf("Fallback export done: %s", progress.TableName)

	return nil
}

//
// ---------- HELPERS ----------
//

func (re *ResumableExporter) getPrimaryKey(table string, columns []string) (string, int, error) {

	query := `
		SELECT COLUMN_NAME 
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		AND COLUMN_KEY = 'PRI'
		LIMIT 1
	`

	var pk string
	err := re.db.QueryRow(query, table).Scan(&pk)
	if err != nil {
		return "", -1, err
	}

	for i, c := range columns {
		if c == pk {
			return pk, i, nil
		}
	}

	return "", -1, fmt.Errorf("pk index not found")
}

func (re *ResumableExporter) buildValuesOnly(values []interface{}, columnTypes []*sql.ColumnType) string {
	out := make([]string, len(values))

	for i, val := range values {
		if val == nil {
			out[i] = "NULL"
			continue
		}

		colType := strings.ToLower(columnTypes[i].DatabaseTypeName())

		switch v := val.(type) {
		case string:
			s := strings.ReplaceAll(v, "\\", "\\\\")
			s = strings.ReplaceAll(s, "'", "''")
			out[i] = fmt.Sprintf("'%s'", s)

		case []byte:
			if strings.Contains(colType, "blob") || strings.Contains(colType, "binary") {
				out[i] = fmt.Sprintf("FROM_BASE64('%s')", base64.StdEncoding.EncodeToString(v))
			} else {
				s := string(v)
				s = strings.ReplaceAll(s, "\\", "\\\\")
				s = strings.ReplaceAll(s, "'", "''")
				out[i] = fmt.Sprintf("'%s'", s)
			}

		case int64:
			out[i] = fmt.Sprintf("%d", v)

		case float64:
			out[i] = fmt.Sprintf("%v", v)

		case bool:
			if v {
				out[i] = "1"
			} else {
				out[i] = "0"
			}

		default:
			s := fmt.Sprintf("%v", v)
			s = strings.ReplaceAll(s, "\\", "\\\\")
			s = strings.ReplaceAll(s, "'", "''")
			out[i] = fmt.Sprintf("'%s'", s)
		}
	}

	return fmt.Sprintf("(%s)", strings.Join(out, ", "))
}

func (re *ResumableExporter) writeBulkInsert(writer *bufio.Writer, table string, columns []string, values []string) {
	colList := make([]string, len(columns))
	for i, c := range columns {
		colList[i] = fmt.Sprintf("`%s`", c)
	}

	stmt := fmt.Sprintf(
		"INSERT INTO `%s` (%s) VALUES\n%s;\n",
		table,
		strings.Join(colList, ", "),
		strings.Join(values, ",\n"),
	)

	writer.WriteString(stmt)
}

func (re *ResumableExporter) loadProgress(table string) *ExportProgress {
	file := fmt.Sprintf("%s/%s_progress.json", re.progressDir, table)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var p ExportProgress
	if json.Unmarshal(data, &p) != nil {
		return nil
	}
	return &p
}

func (re *ResumableExporter) saveProgress(p *ExportProgress) {
	file := fmt.Sprintf("%s/%s_progress.json", re.progressDir, p.TableName)
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(file, data, 0644)
}

func (re *ResumableExporter) getTableColumns(table string) ([]string, []*sql.ColumnType, error) {
	rows, err := re.db.Query(fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", table))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	ct, _ := rows.ColumnTypes()
	cols := make([]string, len(ct))
	for i, c := range ct {
		cols[i] = c.Name()
	}
	return cols, ct, nil
}

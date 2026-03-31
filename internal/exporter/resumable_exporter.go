// internal/exporter/resumable_exporter.go
package exporter

import (
	"bufio"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ExportProgress struct {
	TableName      string    `json:"table_name"`
	LastExportedID int64     `json:"last_exported_id"`
	RowsExported   int64     `json:"rows_exported"`
	TotalRows      int64     `json:"total_rows"`
	StartTime      time.Time `json:"start_time"`
	LastUpdate     time.Time `json:"last_update"`
	Status         string    `json:"status"` // running, paused, completed, failed
	FileName       string    `json:"file_name"`
}

type ResumableExporter struct {
	db          *sql.DB
	config      *ExporterConfig
	progressDir string
}

func NewResumableExporter(db *sql.DB, config *ExporterConfig, progressDir string) *ResumableExporter {
	// Create progress directory if it doesn't exist
	os.MkdirAll(progressDir, 0755)

	return &ResumableExporter{
		db:          db,
		config:      config,
		progressDir: progressDir,
	}
}

func (re *ResumableExporter) ExportWithResume(tableName string) error {
	// Check for existing progress
	progress := re.loadProgress(tableName)

	if progress != nil && progress.Status == "running" {
		log.Printf("Resuming export of %s from row %d (already exported %d rows)",
			tableName, progress.LastExportedID, progress.RowsExported)
		return re.resumeExport(progress)
	}

	// Start fresh export
	log.Printf("Starting fresh export of %s", tableName)
	return re.startNewExport(tableName)
}

func (re *ResumableExporter) startNewExport(tableName string) error {
	// Get total rows
	var totalRows int64
	err := re.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)).Scan(&totalRows)
	if err != nil {
		return err
	}

	if totalRows == 0 {
		log.Printf("Table %s is empty", tableName)
		return nil
	}

	filename := fmt.Sprintf("data_%s_resumable.sql", tableName)
	filePath := filepath.Join(re.config.OutputDir, filename)

	progress := &ExportProgress{
		TableName:      tableName,
		LastExportedID: 0,
		RowsExported:   0,
		TotalRows:      totalRows,
		StartTime:      time.Now(),
		LastUpdate:     time.Now(),
		Status:         "running",
		FileName:       filename,
	}

	re.saveProgress(progress)

	return re.exportInBatches(progress, filePath)
}

func (re *ResumableExporter) resumeExport(progress *ExportProgress) error {
	filename := progress.FileName
	filePath := filepath.Join(re.config.OutputDir, filename)

	// Open file in append mode
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	log.Printf("Resuming export to file: %s", filename)

	return re.exportInBatches(progress, filePath)
}

func (re *ResumableExporter) exportInBatches(progress *ExportProgress, filePath string) error {
	batchSize := re.config.RowsPerBatch

	// Get columns
	columns, columnTypes, err := re.getTableColumns(progress.TableName)
	if err != nil {
		progress.Status = "failed"
		re.saveProgress(progress)
		return err
	}

	// Open file for writing
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024)
	defer writer.Flush()

	// Write header if this is a new file
	if progress.RowsExported == 0 {
		writer.WriteString(fmt.Sprintf("-- Resumable export for table: %s\n", progress.TableName))
		writer.WriteString(fmt.Sprintf("-- Total rows: %d\n", progress.TotalRows))
		writer.WriteString("-- ============================================\n\n")
		writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", progress.TableName))
		writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", progress.TableName))
	}

	startTime := time.Now()
	lastProgressSave := time.Now()

	for progress.RowsExported < progress.TotalRows {
		// Query with cursor
		query := fmt.Sprintf(`
            SELECT * FROM `+"`%s`"+` 
            WHERE id > ? 
            ORDER BY id 
            LIMIT ?
        `, progress.TableName)

		rows, err := re.db.Query(query, progress.LastExportedID, batchSize)
		if err != nil {
			progress.Status = "failed"
			re.saveProgress(progress)
			return err
		}

		batchCount := 0
		var lastID int64

		for rows.Next() {
			// Create value holders
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				progress.Status = "failed"
				re.saveProgress(progress)
				return err
			}

			// Build INSERT statement
			insertStmt := re.buildInsertStatement(progress.TableName, columns, values, columnTypes)
			if _, err := writer.WriteString(insertStmt); err != nil {
				rows.Close()
				return err
			}

			batchCount++

			// Try to get ID from values (assuming first column is ID)
			if id, ok := values[0].(int64); ok {
				lastID = id
			}
		}

		rows.Close()

		if batchCount == 0 {
			break
		}

		// Update progress
		progress.RowsExported += int64(batchCount)
		progress.LastExportedID = lastID
		progress.LastUpdate = time.Now()

		// Save progress every 100,000 rows or every 30 seconds
		if progress.RowsExported%100000 == 0 || time.Since(lastProgressSave) > 30*time.Second {
			re.saveProgress(progress)
			lastProgressSave = time.Now()

			// Calculate ETA
			elapsed := time.Since(startTime)
			rate := float64(progress.RowsExported) / elapsed.Seconds()
			remainingRows := float64(progress.TotalRows - progress.RowsExported)
			eta := time.Duration(remainingRows/rate) * time.Second

			log.Printf("%s: %d/%d rows (%.1f%%) - Rate: %.0f rows/sec - ETA: %v",
				progress.TableName,
				progress.RowsExported,
				progress.TotalRows,
				float64(progress.RowsExported)/float64(progress.TotalRows)*100,
				rate,
				eta)
		}

		// Flush writer periodically
		if progress.RowsExported%10000 == 0 {
			writer.Flush()
		}

		// Add small delay to prevent overwhelming
		if progress.RowsExported > 1000000 {
			time.Sleep(time.Millisecond * 10)
		}
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", progress.TableName))
	writer.WriteString("UNLOCK TABLES;\n")
	writer.Flush()

	progress.Status = "completed"
	re.saveProgress(progress)

	log.Printf("Completed export of %s: %d rows in %v",
		progress.TableName, progress.RowsExported, time.Since(startTime))

	return nil
}

func (re *ResumableExporter) loadProgress(tableName string) *ExportProgress {
	filename := fmt.Sprintf("%s/%s_progress.json", re.progressDir, tableName)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil
	}

	var progress ExportProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil
	}

	return &progress
}

func (re *ResumableExporter) saveProgress(progress *ExportProgress) {
	filename := fmt.Sprintf("%s/%s_progress.json", re.progressDir, progress.TableName)
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		log.Printf("Error saving progress: %v", err)
		return
	}

	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		log.Printf("Error writing progress file: %v", err)
	}
}

func (re *ResumableExporter) getTableColumns(tableName string) ([]string, []*sql.ColumnType, error) {
	query := fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", tableName)
	rows, err := re.db.Query(query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, err
	}

	columns := make([]string, len(columnTypes))
	for i, ct := range columnTypes {
		columns[i] = ct.Name()
	}

	return columns, columnTypes, nil
}

func (re *ResumableExporter) buildInsertStatement(tableName string, columns []string,
	values []interface{}, columnTypes []*sql.ColumnType) string {

	// Build column list
	columnList := make([]string, len(columns))
	for i, col := range columns {
		columnList[i] = fmt.Sprintf("`%s`", col)
	}

	// Build value list
	valueStrings := make([]string, len(values))
	for i, val := range values {
		if val == nil {
			valueStrings[i] = "NULL"
			continue
		}

		colType := columnTypes[i].DatabaseTypeName()

		switch v := val.(type) {
		case string:
			escaped := strings.ReplaceAll(v, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "'", "''")
			valueStrings[i] = fmt.Sprintf("'%s'", escaped)

		case []byte:
			isBinary := strings.Contains(strings.ToLower(colType), "blob") ||
				strings.Contains(strings.ToLower(colType), "binary")

			if isBinary {
				encoded := base64.StdEncoding.EncodeToString(v)
				valueStrings[i] = fmt.Sprintf("FROM_BASE64('%s')", encoded)
			} else {
				str := string(v)
				escaped := strings.ReplaceAll(str, "\\", "\\\\")
				escaped = strings.ReplaceAll(escaped, "'", "''")
				valueStrings[i] = fmt.Sprintf("'%s'", escaped)
			}

		case int, int8, int16, int32, int64:
			valueStrings[i] = fmt.Sprintf("%d", v)

		case uint, uint8, uint16, uint32, uint64:
			valueStrings[i] = fmt.Sprintf("%d", v)

		case float32, float64:
			valueStrings[i] = fmt.Sprintf("%v", v)

		case bool:
			if v {
				valueStrings[i] = "1"
			} else {
				valueStrings[i] = "0"
			}

		default:
			str := fmt.Sprintf("%v", v)
			escaped := strings.ReplaceAll(str, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "'", "''")
			valueStrings[i] = fmt.Sprintf("'%s'", escaped)
		}
	}

	return fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s);\n",
		tableName, strings.Join(columnList, ", "), strings.Join(valueStrings, ", "))
}

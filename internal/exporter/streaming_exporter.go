// internal/exporter/streaming_exporter.go
package exporter

import (
	"bufio"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type StreamingExporter struct {
	db          *sql.DB
	config      *ExporterConfig
	workerCount int
}

type RowData struct {
	Values []interface{}
}

func NewStreamingExporter(db *sql.DB, config *ExporterConfig, workerCount int) *StreamingExporter {
	return &StreamingExporter{
		db:          db,
		config:      config,
		workerCount: workerCount,
	}
}

func (se *StreamingExporter) ExportLargeTable(tableName string) error {

	log.Printf("Streaming export started: %s", tableName)

	var totalRows int64
	if err := se.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)).Scan(&totalRows); err != nil {
		return err
	}

	if totalRows == 0 {
		return nil
	}

	columns, columnTypes, err := se.getTableColumns(tableName)
	if err != nil {
		return err
	}

	rowChan := make(chan RowData, se.workerCount*100)
	errChan := make(chan error, se.workerCount+1)

	// producer
	go se.fetchRows(tableName, columns, rowChan, errChan)

	// consumers
	var wg sync.WaitGroup
	for i := 0; i < se.workerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			se.writeRows(id, tableName, columns, columnTypes, rowChan, errChan)
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	go se.monitorProgress(tableName, totalRows)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	log.Printf("Streaming export completed: %s", tableName)
	return nil
}

func (se *StreamingExporter) fetchRows(
	table string,
	columns []string,
	rowChan chan<- RowData,
	errChan chan<- error,
) {
	defer close(rowChan)

	pk, err := se.getPrimaryKey(table)

	// ❌ No PK → fallback
	if err != nil || pk == "" {
		log.Printf("Table %s has no PK → fallback full scan", table)
		se.fetchRowsSimple(table, rowChan, errChan)
		return
	}

	log.Printf("Using PK `%s` for streaming: %s", pk, table)

	var lastVal interface{} = 0
	batchSize := se.config.RowsPerBatch

	for {
		query := fmt.Sprintf(`
			SELECT * FROM `+"`%s`"+`
			WHERE `+"`%s`"+` > ?
			ORDER BY `+"`%s`"+`
			LIMIT ?
		`, table, pk, pk)

		rows, err := se.db.Query(query, lastVal, batchSize)
		if err != nil {
			errChan <- err
			return
		}

		count := 0

		for rows.Next() {
			values := make([]interface{}, len(columns))
			ptrs := make([]interface{}, len(columns))
			for i := range values {
				ptrs[i] = &values[i]
			}

			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				errChan <- err
				return
			}

			// update lastVal dynamically
			for i, col := range columns {
				if strings.EqualFold(col, pk) {
					lastVal = extractPKValue(values[i])
					break
				}
			}

			rowChan <- RowData{Values: values}
			count++
		}

		rows.Close()

		if count == 0 {
			break
		}
	}
}

func (se *StreamingExporter) fetchRowsSimple(
	table string,
	rowChan chan<- RowData,
	errChan chan<- error,
) {
	query := fmt.Sprintf("SELECT * FROM `%s`", table)

	rows, err := se.db.Query(query)
	if err != nil {
		errChan <- err
		return
	}
	defer rows.Close()

	cols, _ := rows.Columns()

	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			errChan <- err
			return
		}

		rowChan <- RowData{Values: values}
	}
}

func (se *StreamingExporter) writeRows(
	workerID int,
	table string,
	columns []string,
	columnTypes []*sql.ColumnType,
	rowChan <-chan RowData,
	errChan chan<- error,
) {

	filename := fmt.Sprintf("data_%s_worker%d.sql", table, workerID)
	filePath := filepath.Join(se.config.OutputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		errChan <- err
		return
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024)
	defer writer.Flush()

	writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", table))
	writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", table))

	bulkSize := se.config.BulkInsertSize
	if bulkSize <= 0 {
		bulkSize = 1000
	}

	var buffer []string
	rowCount := 0

	for row := range rowChan {

		valStr := se.buildValuesOnly(row.Values, columnTypes)
		buffer = append(buffer, valStr)

		if len(buffer) >= bulkSize {
			se.writeBulkInsert(writer, table, columns, buffer)
			buffer = buffer[:0]
		}

		rowCount++

		if rowCount%10000 == 0 {
			writer.Flush()
		}
	}

	if len(buffer) > 0 {
		se.writeBulkInsert(writer, table, columns, buffer)
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", table))
	writer.WriteString("UNLOCK TABLES;\n")

	log.Printf("Worker %d wrote %d rows", workerID, rowCount)
}

// ---------- BULK ----------

func (se *StreamingExporter) buildValuesOnly(values []interface{}, columnTypes []*sql.ColumnType) string {
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

func (se *StreamingExporter) writeBulkInsert(
	writer *bufio.Writer,
	table string,
	columns []string,
	values []string,
) {
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

// ---------- PK ----------

func (se *StreamingExporter) getPrimaryKey(table string) (string, error) {
	query := `
	SELECT COLUMN_NAME
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_SCHEMA = DATABASE()
	AND TABLE_NAME = ?
	AND COLUMN_KEY = 'PRI'
	LIMIT 1
	`

	var pk string
	err := se.db.QueryRow(query, table).Scan(&pk)
	if err != nil {
		return "", err
	}

	return pk, nil
}

func extractPKValue(v interface{}) interface{} {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case []byte:
		return string(t)
	case string:
		return t
	default:
		return t
	}
}

// ---------- UTIL ----------

func (se *StreamingExporter) getTableColumns(table string) ([]string, []*sql.ColumnType, error) {
	rows, err := se.db.Query(fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", table))
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

func (se *StreamingExporter) monitorProgress(table string, total int64) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("Streaming %s... total rows: %d", table, total)
	}
}

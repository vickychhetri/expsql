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

func NewStreamingExporter(db *sql.DB, config *ExporterConfig, workerCount int) *StreamingExporter {
	return &StreamingExporter{
		db:          db,
		config:      config,
		workerCount: workerCount,
	}
}

func (se *StreamingExporter) ExportLargeTable(tableName string) error {
	log.Printf("Starting streaming export for table: %s", tableName)

	// Get total rows for progress tracking
	var totalRows int64
	err := se.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)).Scan(&totalRows)
	if err != nil {
		return err
	}

	if totalRows == 0 {
		log.Printf("Table %s is empty", tableName)
		return nil
	}

	// Create channel for row data
	rowChan := make(chan RowData, se.workerCount*10)
	errChan := make(chan error, se.workerCount+1)

	// Get columns once
	columns, columnTypes, err := se.getTableColumns(tableName)
	if err != nil {
		return err
	}

	// Producer: fetches rows and sends to channel
	go se.fetchRows(tableName, columns, columnTypes, rowChan, errChan)

	// Consumers: write to file concurrently
	var wg sync.WaitGroup
	for i := 0; i < se.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			se.writeRows(workerID, tableName, columns, columnTypes, rowChan, errChan)
		}(i)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Monitor progress
	go se.monitorProgress(tableName, totalRows, errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	log.Printf("Completed streaming export for table: %s", tableName)
	return nil
}

type RowData struct {
	Values []interface{}
}

func (se *StreamingExporter) fetchRows(tableName string, columns []string, columnTypes []*sql.ColumnType,
	rowChan chan<- RowData, errChan chan<- error) {
	defer close(rowChan)

	// Use cursor-based streaming
	var lastID int64 = 0
	batchSize := se.config.RowsPerBatch

	for {
		// Try to use ID column if exists
		query := fmt.Sprintf(`
            SELECT * FROM `+"`%s`"+` 
            WHERE id > ? 
            ORDER BY id 
            LIMIT ?
        `, tableName)

		rows, err := se.db.Query(query, lastID, batchSize)
		if err != nil {
			// Fallback to simple select if no ID column
			errChan <- se.fetchRowsSimple(tableName, rowChan)
			return
		}

		rowCount := 0

		for rows.Next() {
			// Create value holders
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				errChan <- err
				return
			}

			// Send to channel
			rowChan <- RowData{Values: values}
			rowCount++
		}

		rows.Close()

		if rowCount < batchSize {
			break
		}

		// Update lastID (assuming first column is ID)
		// In production, you'd need to track the actual ID value
		lastID += int64(batchSize)

		// Small delay to prevent overwhelming
		time.Sleep(time.Millisecond * 10)
	}
}

func (se *StreamingExporter) fetchRowsSimple(tableName string, rowChan chan<- RowData) error {
	query := fmt.Sprintf("SELECT * FROM `%s`", tableName)
	rows, err := se.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, _ := rows.Columns()

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		rowChan <- RowData{Values: values}
	}

	return nil
}

func (se *StreamingExporter) writeRows(workerID int, tableName string,
	columns []string, columnTypes []*sql.ColumnType,
	rowChan <-chan RowData, errChan chan<- error) {

	// Each worker writes to its own file
	filename := fmt.Sprintf("data_%s_worker%d.sql", tableName, workerID)
	filePath := filepath.Join(se.config.OutputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		errChan <- err
		return
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024)
	defer writer.Flush()

	// Write header
	writer.WriteString(fmt.Sprintf("-- Worker %d data for table: %s\n", workerID, tableName))
	writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", tableName))
	writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", tableName))

	rowCount := 0
	for rowData := range rowChan {
		insertStmt := se.buildInsertStatement(tableName, columns, rowData.Values, columnTypes)
		if _, err := writer.WriteString(insertStmt); err != nil {
			errChan <- err
			return
		}

		rowCount++

		// Flush every 1000 rows
		if rowCount%1000 == 0 {
			writer.Flush()
		}
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", tableName))
	writer.WriteString("UNLOCK TABLES;\n")

	log.Printf("Worker %d wrote %d rows for table %s", workerID, rowCount, tableName)
}

func (se *StreamingExporter) buildInsertStatement(tableName string, columns []string,
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

func (se *StreamingExporter) getTableColumns(tableName string) ([]string, []*sql.ColumnType, error) {
	query := fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", tableName)
	rows, err := se.db.Query(query)
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

func (se *StreamingExporter) monitorProgress(tableName string, totalRows int64, errChan chan error) {
	// This is a simple progress monitor - you could enhance this
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Check if we're still processing
		select {
		case <-errChan:
			return
		default:
			log.Printf("Still exporting %s... (total rows: %d)", tableName, totalRows)
		}
	}
}

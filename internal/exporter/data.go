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
)

type DataExporter struct {
	db     *sql.DB
	config *ExporterConfig
}

func NewDataExporter(db *sql.DB, config *ExporterConfig) *DataExporter {
	return &DataExporter{
		db:     db,
		config: config,
	}
}

func (d *DataExporter) ExportTableData(tableName string) error {
	// Get total row count
	var totalRows int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)
	err := d.db.QueryRow(countQuery).Scan(&totalRows)
	if err != nil {
		return fmt.Errorf("failed to count rows in %s: %v", tableName, err)
	}

	if totalRows == 0 {
		log.Printf("Table %s has no data, skipping", tableName)
		return nil
	}

	log.Printf("Table %s has %d rows to export", tableName, totalRows)

	// Create data file for this table
	filename := fmt.Sprintf("data_%s.sql", tableName)
	filePath := filepath.Join(d.config.OutputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024) // 1MB buffer
	defer writer.Flush()

	// Get column names and types
	columns, columnTypes, err := d.getTableColumns(tableName)
	if err != nil {
		return err
	}

	if len(columns) == 0 {
		return fmt.Errorf("no columns found for table %s", tableName)
	}

	// Write header
	writer.WriteString(fmt.Sprintf("-- Data for table: %s\n", tableName))
	writer.WriteString(fmt.Sprintf("-- Total rows: %d\n", totalRows))
	writer.WriteString("-- ============================================\n\n")
	writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", tableName))
	writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", tableName))

	// Export data in batches
	offset := 0
	batchSize := d.config.RowsPerBatch
	rowsExported := 0

	for offset < int(totalRows) {
		query := fmt.Sprintf("SELECT * FROM `%s` LIMIT %d OFFSET %d",
			tableName, batchSize, offset)

		rows, err := d.db.Query(query)
		if err != nil {
			return fmt.Errorf("query failed for table %s: %v", tableName, err)
		}

		// Process batch
		batchCount := 0
		for rows.Next() {
			// Create value holders
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				return fmt.Errorf("scan failed for table %s: %v", tableName, err)
			}

			// Build INSERT statement
			insertStmt := d.buildInsertStatement(tableName, columns, values, columnTypes)
			if _, err := writer.WriteString(insertStmt); err != nil {
				rows.Close()
				return err
			}
			batchCount++
			rowsExported++
		}

		rows.Close()

		if batchCount > 0 {
			writer.WriteString("\n")
		}

		offset += batchSize

		// Flush periodically
		if rowsExported%10000 == 0 {
			writer.Flush()
			log.Printf("  Exported %d/%d rows from %s", rowsExported, totalRows, tableName)
		}
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", tableName))
	writer.WriteString("UNLOCK TABLES;\n")

	log.Printf("Completed exporting %d rows from %s", rowsExported, tableName)

	return nil
}

func (d *DataExporter) getTableColumns(tableName string) ([]string, []*sql.ColumnType, error) {
	query := fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", tableName)
	rows, err := d.db.Query(query)
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

func (d *DataExporter) buildInsertStatement(tableName string, columns []string, values []interface{}, columnTypes []*sql.ColumnType) string {
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

		// Get column type for proper formatting
		colType := columnTypes[i].DatabaseTypeName()

		switch v := val.(type) {
		case string:
			// Escape single quotes and backslashes
			escaped := strings.ReplaceAll(v, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "'", "''")
			valueStrings[i] = fmt.Sprintf("'%s'", escaped)

		case []byte:
			// Check if it's a text type or binary
			isBinary := strings.Contains(strings.ToLower(colType), "blob") ||
				strings.Contains(strings.ToLower(colType), "binary")

			if isBinary {
				// Encode binary data as base64
				encoded := base64.StdEncoding.EncodeToString(v)
				valueStrings[i] = fmt.Sprintf("FROM_BASE64('%s')", encoded)
			} else {
				// Treat as string
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
			// For other types, convert to string
			str := fmt.Sprintf("%v", v)
			escaped := strings.ReplaceAll(str, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "'", "''")
			valueStrings[i] = fmt.Sprintf("'%s'", escaped)
		}
	}

	return fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s);\n",
		tableName, strings.Join(columnList, ", "), strings.Join(valueStrings, ", "))
}

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
	// Count rows
	var totalRows int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)
	if err := d.db.QueryRow(countQuery).Scan(&totalRows); err != nil {
		return fmt.Errorf("failed to count rows in %s: %v", tableName, err)
	}

	if totalRows == 0 {
		log.Printf("Table %s has no data, skipping", tableName)
		return nil
	}

	log.Printf("Table %s has %d rows to export", tableName, totalRows)

	// File setup
	filename := fmt.Sprintf("data_%s.sql", tableName)
	filePath := filepath.Join(d.config.OutputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024)
	defer writer.Flush()

	// Get columns
	columns, columnTypes, err := d.getTableColumns(tableName)
	if err != nil {
		return err
	}

	if len(columns) == 0 {
		return fmt.Errorf("no columns found for table %s", tableName)
	}

	// Header
	writer.WriteString(fmt.Sprintf("-- Data for table: %s\n", tableName))
	writer.WriteString(fmt.Sprintf("-- Total rows: %d\n", totalRows))
	writer.WriteString("-- ============================================\n\n")
	writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", tableName))
	writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", tableName))

	// Bulk config
	batchSize := d.config.RowsPerBatch
	bulkSize := d.config.BulkInsertSize
	if bulkSize <= 0 {
		bulkSize = 1000
	}

	offset := 0
	rowsExported := 0

	for offset < int(totalRows) {
		query := fmt.Sprintf("SELECT * FROM `%s` LIMIT %d OFFSET %d",
			tableName, batchSize, offset)

		rows, err := d.db.Query(query)
		if err != nil {
			return fmt.Errorf("query failed for table %s: %v", tableName, err)
		}

		var valueBuffer []string
		batchCount := 0

		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				return fmt.Errorf("scan failed for table %s: %v", tableName, err)
			}

			valueStr := d.buildValuesOnly(values, columnTypes)
			valueBuffer = append(valueBuffer, valueStr)

			// Flush bulk insert
			if len(valueBuffer) >= bulkSize {
				d.writeBulkInsert(writer, tableName, columns, valueBuffer)
				valueBuffer = valueBuffer[:0]
			}

			batchCount++
			rowsExported++
		}

		rows.Close()

		// Flush remaining
		if len(valueBuffer) > 0 {
			d.writeBulkInsert(writer, tableName, columns, valueBuffer)
			valueBuffer = valueBuffer[:0]
		}

		offset += batchSize

		if rowsExported%10000 == 0 {
			writer.Flush()
			log.Printf("Exported %d/%d rows from %s", rowsExported, totalRows, tableName)
		}

		if batchCount > 0 {
			writer.WriteString("\n")
		}
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", tableName))
	writer.WriteString("UNLOCK TABLES;\n")

	log.Printf("Completed exporting %d rows from %s", rowsExported, tableName)

	return nil
}

// -------------------- HELPERS --------------------

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

func (d *DataExporter) buildValuesOnly(values []interface{}, columnTypes []*sql.ColumnType) string {
	valueStrings := make([]string, len(values))

	for i, val := range values {
		if val == nil {
			valueStrings[i] = "NULL"
			continue
		}

		colType := strings.ToLower(columnTypes[i].DatabaseTypeName())

		switch v := val.(type) {
		case string:
			escaped := strings.ReplaceAll(v, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "'", "''")
			valueStrings[i] = fmt.Sprintf("'%s'", escaped)

		case []byte:
			isBinary := strings.Contains(colType, "blob") || strings.Contains(colType, "binary")

			if isBinary {
				encoded := base64.StdEncoding.EncodeToString(v)
				valueStrings[i] = fmt.Sprintf("FROM_BASE64('%s')", encoded)
			} else {
				str := string(v)
				escaped := strings.ReplaceAll(str, "\\", "\\\\")
				escaped = strings.ReplaceAll(escaped, "'", "''")
				valueStrings[i] = fmt.Sprintf("'%s'", escaped)
			}

		case int64:
			valueStrings[i] = fmt.Sprintf("%d", v)

		case float64:
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

	return fmt.Sprintf("(%s)", strings.Join(valueStrings, ", "))
}

func (d *DataExporter) writeBulkInsert(
	writer *bufio.Writer,
	tableName string,
	columns []string,
	values []string,
) {
	columnList := make([]string, len(columns))
	for i, col := range columns {
		columnList[i] = fmt.Sprintf("`%s`", col)
	}

	stmt := fmt.Sprintf(
		"INSERT INTO `%s` (%s) VALUES\n%s;\n",
		tableName,
		strings.Join(columnList, ", "),
		strings.Join(values, ",\n"),
	)

	writer.WriteString(stmt)
}

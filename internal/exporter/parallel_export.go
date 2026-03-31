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
)

type ParallelTableExporter struct {
	db         *sql.DB
	config     *ExporterConfig
	tableName  string
	partitions int
	pkColumn   string // Primary key column name
	pkType     string // Primary key type (int, bigint, etc)
}

func NewParallelTableExporter(db *sql.DB, config *ExporterConfig, tableName string, partitions int) *ParallelTableExporter {
	return &ParallelTableExporter{
		db:         db,
		config:     config,
		tableName:  tableName,
		partitions: partitions,
	}
}

func (pte *ParallelTableExporter) ExportTableDataParallel() error {
	// Detect primary key column
	pkColumn, pkType, err := pte.getPrimaryKeyInfo()
	if err != nil {
		log.Printf("Table %s: error detecting primary key: %v, using regular export", pte.tableName, err)
		return pte.fallbackExport()
	}

	if pkColumn == "" {
		log.Printf("Table %s doesn't have a primary key, using regular export", pte.tableName)
		return pte.fallbackExport()
	}

	// Check if primary key is numeric (required for range partitioning)
	if !strings.Contains(strings.ToLower(pkType), "int") &&
		!strings.Contains(strings.ToLower(pkType), "bigint") &&
		!strings.Contains(strings.ToLower(pkType), "smallint") &&
		!strings.Contains(strings.ToLower(pkType), "tinyint") &&
		!strings.Contains(strings.ToLower(pkType), "mediumint") {
		log.Printf("Table %s has primary key '%s' but type '%s' is not numeric, using regular export",
			pte.tableName, pkColumn, pkType)
		return pte.fallbackExport()
	}

	pte.pkColumn = pkColumn
	pte.pkType = pkType
	log.Printf("Table %s has numeric primary key: %s (%s), using parallel export",
		pte.tableName, pkColumn, pkType)

	// Get min/max values for partitioning
	var minVal, maxVal int64
	query := fmt.Sprintf("SELECT MIN(`%s`), MAX(`%s`) FROM `%s`", pkColumn, pkColumn, pte.tableName)
	err = pte.db.QueryRow(query).Scan(&minVal, &maxVal)
	if err != nil {
		log.Printf("Table %s: failed to get min/max of %s: %v, using regular export",
			pte.tableName, pkColumn, err)
		return pte.fallbackExport()
	}

	if minVal == 0 && maxVal == 0 {
		// Check if table is actually empty
		var count int64
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", pte.tableName)
		pte.db.QueryRow(countQuery).Scan(&count)
		if count == 0 {
			log.Printf("Table %s has no data", pte.tableName)
			return nil
		}
	}

	log.Printf("Table %s: %s range from %d to %d", pte.tableName, pkColumn, minVal, maxVal)

	// Calculate partition ranges
	valueRange := maxVal - minVal
	if valueRange == 0 {
		log.Printf("Table %s has only one distinct %s value, using regular export", pte.tableName, pkColumn)
		return pte.fallbackExport()
	}

	partitionSize := valueRange / int64(pte.partitions)

	if partitionSize == 0 {
		// If range is too small, use fewer partitions
		pte.partitions = int(valueRange) + 1
		if pte.partitions > 10 {
			pte.partitions = 10
		}
		partitionSize = valueRange / int64(pte.partitions)
		if partitionSize == 0 {
			partitionSize = 1
		}
	}

	log.Printf("Partitioning table %s into %d chunks (%s range: %d - %d, size: %d)",
		pte.tableName, pte.partitions, pkColumn, minVal, maxVal, partitionSize)

	// Create worker pool for partitions
	var wg sync.WaitGroup
	errChan := make(chan error, pte.partitions)

	// Get columns once for all partitions
	columns, columnTypes, err := pte.getTableColumns()
	if err != nil {
		return err
	}

	for i := 0; i < pte.partitions; i++ {
		startVal := minVal + (int64(i) * partitionSize)
		endVal := minVal + (int64(i+1) * partitionSize)

		if i == pte.partitions-1 {
			endVal = maxVal
		}

		// Adjust for the first partition
		if i == 0 {
			startVal = minVal
		}

		wg.Add(1)
		go func(partition int, start, end int64) {
			defer wg.Done()

			filename := fmt.Sprintf("data_%s_part%d.sql", pte.tableName, partition)
			log.Printf("Partition %d: exporting rows where %s between %d and %d",
				partition, pte.pkColumn, start, end)

			if err := pte.exportPartition(start, end, filename, columns, columnTypes); err != nil {
				errChan <- fmt.Errorf("partition %d failed: %v", partition, err)
			}
		}(i, startVal, endVal)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (pte *ParallelTableExporter) getPrimaryKeyInfo() (string, string, error) {
	query := `
		SELECT k.column_name, c.data_type
		FROM information_schema.table_constraints t
		JOIN information_schema.key_column_usage k
			ON t.constraint_name = k.constraint_name
			AND t.table_schema = k.table_schema
		JOIN information_schema.columns c
			ON k.table_schema = c.table_schema
			AND k.table_name = c.table_name
			AND k.column_name = c.column_name
		WHERE t.constraint_type = 'PRIMARY KEY'
		AND t.table_schema = ?
		AND t.table_name = ?
		LIMIT 1
	`

	var pkColumn, dataType string
	err := pte.db.QueryRow(query, pte.config.DatabaseName, pte.tableName).Scan(&pkColumn, &dataType)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil // No primary key found
		}
		return "", "", err
	}

	return pkColumn, dataType, nil
}

func (pte *ParallelTableExporter) exportPartition(startVal, endVal int64, filename string,
	columns []string, columnTypes []*sql.ColumnType) error {

	filePath := filepath.Join(pte.config.OutputDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024)
	defer writer.Flush()

	// Write header
	writer.WriteString(fmt.Sprintf("-- Partition: %s\n", filename))
	writer.WriteString(fmt.Sprintf("-- %s Range: %d - %d\n", pte.pkColumn, startVal, endVal))
	writer.WriteString("-- ============================================\n\n")
	writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", pte.tableName))
	writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", pte.tableName))

	// Query with WHERE clause using the primary key
	query := fmt.Sprintf(`
		SELECT * FROM `+"`%s`"+` 
		WHERE `+"`%s`"+` >= ? AND `+"`%s`"+` <= ? 
		ORDER BY `+"`%s`"+`
	`, pte.tableName, pte.pkColumn, pte.pkColumn, pte.pkColumn)

	rows, err := pte.db.Query(query, startVal, endVal)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Process rows
	rowCount := 0

	for rows.Next() {
		// Create value holders
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		// Build INSERT statement
		insertStmt := pte.buildInsertStatement(columns, values, columnTypes)
		if _, err := writer.WriteString(insertStmt); err != nil {
			return err
		}

		rowCount++

		// Flush every 1000 rows
		if rowCount%1000 == 0 {
			writer.Flush()
		}

		// Log progress for this partition
		if rowCount%10000 == 0 {
			log.Printf("  Partition %s: exported %d rows", filename, rowCount)
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", pte.tableName))
	writer.WriteString("UNLOCK TABLES;\n")

	if rowCount > 0 {
		log.Printf("Partition %s completed: %d rows", filename, rowCount)
	}
	return nil
}

func (pte *ParallelTableExporter) fallbackExport() error {
	// Fallback to regular export using DataExporter
	log.Printf("Using standard export for table %s", pte.tableName)
	dataExporter := NewDataExporter(pte.db, pte.config)
	return dataExporter.ExportTableData(pte.tableName)
}

func (pte *ParallelTableExporter) getTableColumns() ([]string, []*sql.ColumnType, error) {
	query := fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", pte.tableName)
	rows, err := pte.db.Query(query)
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

func (pte *ParallelTableExporter) buildInsertStatement(columns []string, values []interface{},
	columnTypes []*sql.ColumnType) string {

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
		pte.tableName, strings.Join(columnList, ", "), strings.Join(valueStrings, ", "))
}

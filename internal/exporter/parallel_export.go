package exporter

import (
	"bufio"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ParallelTableExporter struct {
	db         *sql.DB
	config     *ExporterConfig
	tableName  string
	partitions int
	pkColumn   string
	pkType     string
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

	// ✅ Detect PK
	pkColumn, pkType, err := pte.getPrimaryKeyInfo()
	if err != nil || pkColumn == "" {
		log.Printf("Fallback export for %s", pte.tableName)
		return pte.fallbackExport()
	}

	// ✅ COUNT check (SMALL TABLE OPTIMIZATION)
	var totalRows int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", pte.tableName)

	err = pte.db.QueryRow(countQuery).Scan(&totalRows)
	if err != nil {
		log.Printf("Table %s: count failed: %v, fallback export", pte.tableName, err)
		return pte.fallbackExport()
	}

	threshold := pte.config.SmallTableThreshold
	if threshold == 0 {
		threshold = 1000
	}

	if totalRows < int64(threshold) {
		log.Printf("Table %s has only %d rows (<%d), using single export",
			pte.tableName, totalRows, threshold)
		return pte.fallbackExport()
	}

	// ✅ Only numeric PK allowed
	if !strings.Contains(strings.ToLower(pkType), "int") {
		log.Printf("Non-numeric PK, fallback %s", pte.tableName)
		return pte.fallbackExport()
	}

	pte.pkColumn = pkColumn
	pte.pkType = pkType

	// ✅ Get min/max
	var minVal, maxVal int64
	query := fmt.Sprintf("SELECT MIN(`%s`), MAX(`%s`) FROM `%s`", pkColumn, pkColumn, pte.tableName)
	if err := pte.db.QueryRow(query).Scan(&minVal, &maxVal); err != nil {
		return pte.fallbackExport()
	}

	if minVal == maxVal {
		return pte.fallbackExport()
	}

	rangeSize := maxVal - minVal

	// ✅ Adaptive partitioning (better than static)
	if pte.partitions <= 0 {
		pte.partitions = pte.config.Workers
	}

	partitionSize := rangeSize / int64(pte.partitions)
	if partitionSize == 0 {
		partitionSize = 1
	}

	log.Printf("Table %s partitioned into %d chunks (%d - %d)",
		pte.tableName, pte.partitions, minVal, maxVal)

	columns, columnTypes, err := pte.getTableColumns()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, pte.partitions)
	sem := make(chan struct{}, pte.config.Workers)

	for i := 0; i < pte.partitions; i++ {
		start := minVal + int64(i)*partitionSize
		end := start + partitionSize - 1

		if i == pte.partitions-1 {
			end = maxVal
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(part int, s, e int64) {
			defer wg.Done()
			defer func() { <-sem }()

			filename := fmt.Sprintf("data_%s_part%d.sql", pte.tableName, part)

			log.Printf("Partition %d: %s BETWEEN %d AND %d",
				part, pte.pkColumn, s, e)

			err := pte.exportPartition(s, e, filename, columns, columnTypes)
			if err != nil {
				errChan <- fmt.Errorf("partition %d failed: %v", part, err)
			}
		}(i, start, end)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (pte *ParallelTableExporter) exportPartition(
	startVal, endVal int64,
	filename string,
	columns []string,
	columnTypes []*sql.ColumnType,
) error {

	filePath := filepath.Join(pte.config.OutputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1024*1024)
	defer writer.Flush()

	writer.WriteString(fmt.Sprintf("LOCK TABLES `%s` WRITE;\n", pte.tableName))
	writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n\n", pte.tableName))

	bulkSize := pte.config.BulkInsertSize
	if bulkSize <= 0 {
		bulkSize = 1000
	}

	var valueBuffer []string

	current := startVal
	batchSize := int64(50000)

	pkIndex := pte.getPKIndex(columns)
	if pkIndex == -1 {
		return fmt.Errorf("pk not found")
	}

	for current <= endVal {

		query := fmt.Sprintf(`
			SELECT * FROM `+"`%s`"+`
			WHERE `+"`%s`"+` >= ? AND `+"`%s`"+` <= ?
			ORDER BY `+"`%s`"+`
			LIMIT ?
		`, pte.tableName, pte.pkColumn, pte.pkColumn, pte.pkColumn)

		rows, err := pte.db.Query(query, current, endVal, batchSize)
		if err != nil {
			return err
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
				return err
			}

			// update cursor
			switch v := values[pkIndex].(type) {
			case int64:
				current = v + 1
			case []byte:
				val, _ := strconv.ParseInt(string(v), 10, 64)
				current = val + 1
			case string:
				val, _ := strconv.ParseInt(v, 10, 64)
				current = val + 1
			}

			valueStr := pte.buildValuesOnly(values, columnTypes)
			valueBuffer = append(valueBuffer, valueStr)

			if len(valueBuffer) >= bulkSize {
				pte.writeBulkInsert(writer, columns, valueBuffer)
				valueBuffer = valueBuffer[:0]
			}

			count++
		}

		rows.Close()

		if count == 0 {
			break
		}
	}

	// flush remaining
	if len(valueBuffer) > 0 {
		pte.writeBulkInsert(writer, columns, valueBuffer)
	}

	writer.WriteString(fmt.Sprintf("\n/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n", pte.tableName))
	writer.WriteString("UNLOCK TABLES;\n")

	log.Printf("Completed partition %s", filename)

	return nil
}

func (pte *ParallelTableExporter) buildValuesOnly(values []interface{}, columnTypes []*sql.ColumnType) string {
	out := make([]string, len(values))

	for i, val := range values {
		if val == nil {
			out[i] = "NULL"
			continue
		}

		colType := strings.ToLower(columnTypes[i].DatabaseTypeName())

		switch v := val.(type) {
		case time.Time:
			out[i] = fmt.Sprintf("'%s'", v.UTC().Format("2006-01-02 15:04:05"))

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

func (pte *ParallelTableExporter) writeBulkInsert(writer *bufio.Writer, columns []string, values []string) {
	colList := make([]string, len(columns))
	for i, c := range columns {
		colList[i] = fmt.Sprintf("`%s`", c)
	}

	stmt := fmt.Sprintf(
		"INSERT INTO `%s` (%s) VALUES\n%s;\n",
		pte.tableName,
		strings.Join(colList, ", "),
		strings.Join(values, ",\n"),
	)

	writer.WriteString(stmt)
}

// ---------- helpers ----------

func (pte *ParallelTableExporter) getPKIndex(columns []string) int {
	for i, c := range columns {
		if strings.EqualFold(c, pte.pkColumn) {
			return i
		}
	}
	return -1
}

func (pte *ParallelTableExporter) fallbackExport() error {
	dataExporter := NewDataExporter(pte.db, pte.config)
	return dataExporter.ExportTableData(pte.tableName)
}

func (pte *ParallelTableExporter) getTableColumns() ([]string, []*sql.ColumnType, error) {
	rows, err := pte.db.Query(fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", pte.tableName))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cts, _ := rows.ColumnTypes()
	cols := make([]string, len(cts))
	for i, ct := range cts {
		cols[i] = ct.Name()
	}

	return cols, cts, nil
}

func (pte *ParallelTableExporter) getPrimaryKeyInfo() (string, string, error) {
	query := `
	SELECT COLUMN_NAME, DATA_TYPE
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_SCHEMA = ?
	AND TABLE_NAME = ?
	AND COLUMN_KEY = 'PRI'
	LIMIT 1
	`

	var col, typ string
	err := pte.db.QueryRow(query, pte.config.DatabaseName, pte.tableName).Scan(&col, &typ)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return col, typ, err
}

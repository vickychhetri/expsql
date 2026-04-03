package exporter

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

type AdvancedConfig struct {
	Strategy    string // auto, parallel, streaming, standard
	Partitions  int
	Resumable   bool
	ProgressDir string
}

type AdvancedExporter struct {
	db           *sql.DB
	config       *ExporterConfig
	advConfig    *AdvancedConfig
	databaseName string
}

func NewAdvancedExporter(dsn string, config *ExporterConfig, advConfig *AdvancedConfig) (*AdvancedExporter, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.Workers * 2)
	db.SetMaxIdleConns(config.Workers)
	// db.SetConnMaxLifetime(0)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Get database name from the connection
	var dbName string
	err = db.QueryRow("SELECT DATABASE()").Scan(&dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get database name: %v", err)
	}

	if dbName == "" {
		return nil, fmt.Errorf("no database selected. Please check your connection string")
	}

	log.Printf("Connected to database: %s", dbName)

	// Set default strategy
	if advConfig.Strategy == "" {
		advConfig.Strategy = "auto"
	}

	// Set partitions to workers if not specified
	if advConfig.Partitions == 0 {
		advConfig.Partitions = config.Workers
	}

	// IMPORTANT: Set the database name in the config
	config.DatabaseName = dbName

	return &AdvancedExporter{
		db:           db,
		config:       config,
		advConfig:    advConfig,
		databaseName: dbName,
	}, nil
}

func (ae *AdvancedExporter) Close() error {
	return ae.db.Close()
}

func (ae *AdvancedExporter) Export() error {
	log.Printf("Exporting database: %s", ae.databaseName)
	log.Printf("Strategy: %s, Workers: %d, Partitions: %d, Batch size: %d",
		ae.advConfig.Strategy, ae.config.Workers, ae.advConfig.Partitions, ae.config.RowsPerBatch)

	// Export design first if requested
	if ae.config.IncludeDesign {
		log.Println("\n=== Starting Design Export ===")
		if err := ae.exportDesign(); err != nil {
			return fmt.Errorf("design export failed: %v", err)
		}
		log.Println("✓ Design export completed")
	}

	// Export data if requested
	if ae.config.IncludeData {
		log.Println("\n=== Starting Data Export ===")
		if err := ae.exportData(); err != nil {
			return fmt.Errorf("data export failed: %v", err)
		}
		log.Println("✓ Data export completed")
	}

	return nil
}

func (ae *AdvancedExporter) exportDesign() error {
	designExporter := NewDesignExporter(ae.db, ae.config.OutputDir, ae.config.Compress, ae.databaseName)

	// Get all tables
	tables, err := ae.getTables()
	if err != nil {
		return err
	}

	if len(tables) == 0 {
		log.Printf("⚠ No tables found in database '%s'", ae.databaseName)

		// Debug: Check if we can see any tables
		debugQuery := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?"
		var count int
		err = ae.db.QueryRow(debugQuery, ae.databaseName).Scan(&count)
		if err != nil {
			log.Printf("Debug: Error counting tables: %v", err)
		} else {
			log.Printf("Debug: information_schema shows %d tables in %s", count, ae.databaseName)
		}

		// List all databases to help debug
		ae.listDatabases()
		return nil
	}

	log.Printf("Found %d tables to export", len(tables))
	for i, table := range tables {
		log.Printf("  %d. %s", i+1, table)
	}

	// Export tables DDL
	if err := designExporter.ExportTables(tables); err != nil {
		return err
	}

	// Export views
	if err := designExporter.ExportViews(); err != nil {
		return err
	}

	// Export functions
	if err := designExporter.ExportFunctions(); err != nil {
		return err
	}

	// Export procedures
	if err := designExporter.ExportProcedures(); err != nil {
		return err
	}

	// Export triggers
	if err := designExporter.ExportTriggers(); err != nil {
		return err
	}

	// Export events
	if err := designExporter.ExportEvents(); err != nil {
		return err
	}

	return nil
}

func (ae *AdvancedExporter) exportData() error {
	tables, err := ae.getTables()
	if err != nil {
		return err
	}

	tables = ae.filterTables(tables)

	if len(tables) == 0 {
		log.Println("No tables to export data from")
		return nil
	}

	log.Printf("Processing %d tables for data export", len(tables))

	for i, table := range tables {
		// Get table size
		rowCount, err := ae.getTableRowCount(table)
		if err != nil {
			return err
		}

		if rowCount == 0 {
			log.Printf("[%d/%d] Table %s: empty, skipping", i+1, len(tables), table)
			continue
		}

		log.Printf("\n[%d/%d] Processing table: %s (%d rows)", i+1, len(tables), table, rowCount)

		// Choose export strategy based on table size and user preference
		strategy := ae.chooseStrategy(table, rowCount)

		startTime := time.Now()

		switch strategy {
		case "parallel":
			log.Printf("  → Using PARALLEL export strategy")
			parallelExporter := NewParallelTableExporter(ae.db, ae.config, table, ae.advConfig.Partitions)
			if err := parallelExporter.ExportTableDataParallel(); err != nil {
				return err
			}

		case "streaming":
			log.Printf("  → Using STREAMING export strategy")
			streamingExporter := NewStreamingExporter(ae.db, ae.config, ae.config.Workers)
			if err := streamingExporter.ExportLargeTable(table); err != nil {
				return err
			}

		case "resumable":
			log.Printf("  → Using RESUMABLE export strategy")
			resumableExporter := NewResumableExporter(ae.db, ae.config, ae.advConfig.ProgressDir)
			if err := resumableExporter.ExportWithResume(table); err != nil {
				return err
			}

		default: // standard
			log.Printf("  → Using STANDARD export strategy")
			dataExporter := NewDataExporter(ae.db, ae.config)
			if err := dataExporter.ExportTableData(table); err != nil {
				return err
			}
		}

		elapsed := time.Since(startTime)
		if rowCount > 0 {
			rate := float64(rowCount) / elapsed.Seconds()
			log.Printf("  ✓ Completed %s: %d rows in %v (%.0f rows/sec)",
				table, rowCount, elapsed, rate)
		}
	}

	return nil
}

func (ae *AdvancedExporter) getTables() ([]string, error) {
	// Use the actual database name from the connection
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = ? 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	log.Printf("Querying for tables in database: %s", ae.databaseName)

	rows, err := ae.db.Query(query, ae.databaseName)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Printf("Found %d tables in database %s", len(tables), ae.databaseName)
	return tables, nil
}

func (ae *AdvancedExporter) getTableRowCount(tableName string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)
	err := ae.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rows in %s: %v", tableName, err)
	}
	return count, nil
}

func (ae *AdvancedExporter) filterTables(tables []string) []string {
	if len(ae.config.Tables) > 0 {
		// Include only specified tables
		tableMap := make(map[string]bool)
		for _, t := range ae.config.Tables {
			tableMap[t] = true
		}

		var filtered []string
		for _, t := range tables {
			if tableMap[t] {
				filtered = append(filtered, t)
			}
		}
		return filtered
	}

	if len(ae.config.ExcludeTables) > 0 {
		// Exclude specified tables
		excludeMap := make(map[string]bool)
		for _, t := range ae.config.ExcludeTables {
			excludeMap[t] = true
		}

		var filtered []string
		for _, t := range tables {
			if !excludeMap[t] {
				filtered = append(filtered, t)
			}
		}
		return filtered
	}

	return tables
}

func (ae *AdvancedExporter) chooseStrategy(tableName string, rowCount int64) string {
	// User specified strategy
	if ae.advConfig.Strategy != "auto" {
		return ae.advConfig.Strategy
	}

	// Auto-select based on table size
	switch {
	case rowCount > 20000000: // > 20 million rows
		if ae.advConfig.Resumable {
			return "resumable"
		}
		return "parallel"

	case rowCount > 5000000: // 5-20 million rows
		if ae.advConfig.Resumable {
			return "resumable"
		}
		return "streaming"

	case rowCount > 1000000: // 1-5 million rows
		return "streaming"

	default: // < 1 million rows
		return "standard"
	}
}

func (ae *AdvancedExporter) listDatabases() {
	rows, err := ae.db.Query("SHOW DATABASES")
	if err != nil {
		log.Printf("Could not list databases: %v", err)
		return
	}
	defer rows.Close()

	log.Println("Available databases:")
	for rows.Next() {
		var db string
		if err := rows.Scan(&db); err != nil {
			continue
		}
		log.Printf("  - %s", db)
	}
}

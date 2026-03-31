package exporter

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

type ExporterConfig struct {
	OutputDir     string
	Workers       int
	RowsPerBatch  int
	Compress      bool
	IncludeData   bool
	IncludeDesign bool
	Tables        []string
	ExcludeTables []string
	DatabaseName  string // Add this field
}

type Exporter struct {
	db     *sql.DB
	config *ExporterConfig
}

func NewExporter(dsn string, config *ExporterConfig) (*Exporter, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Configure connection pool for high concurrency
	db.SetMaxOpenConns(config.Workers * 2)
	db.SetMaxIdleConns(config.Workers)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &Exporter{
		db:     db,
		config: config,
	}, nil
}

func (e *Exporter) Close() error {
	return e.db.Close()
}

func (e *Exporter) Export() error {
	// First, verify database exists and we can query it
	var dbName string
	err := e.db.QueryRow("SELECT DATABASE()").Scan(&dbName)
	if err != nil {
		return fmt.Errorf("failed to get current database: %v", err)
	}

	if dbName == "" {
		return fmt.Errorf("no database selected. Please ensure the database name is correct in the connection string")
	}

	log.Printf("Connected to database: %s", dbName)
	e.config.DatabaseName = dbName

	// Export design first if requested
	if e.config.IncludeDesign {
		log.Println("Starting design export...")
		if err := e.exportDesign(); err != nil {
			return fmt.Errorf("design export failed: %v", err)
		}
		log.Println("Design export completed")
	}

	// Export data if requested
	if e.config.IncludeData {
		log.Println("Starting data export...")
		if err := e.exportData(); err != nil {
			return fmt.Errorf("data export failed: %v", err)
		}
		log.Println("Data export completed")
	}

	return nil
}

func (e *Exporter) exportDesign() error {
	designExporter := NewDesignExporter(e.db, e.config.OutputDir, e.config.Compress, e.config.DatabaseName)

	// Get all tables
	tables, err := e.getTables()
	if err != nil {
		return err
	}

	if len(tables) == 0 {
		log.Println("No tables found in database")
		return nil
	}

	log.Printf("Found %d tables to export", len(tables))

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

func (e *Exporter) exportData() error {
	tables, err := e.getTables()
	if err != nil {
		return err
	}

	// Filter tables
	tables = e.filterTables(tables)

	if len(tables) == 0 {
		log.Println("No tables to export data from")
		return nil
	}

	log.Printf("Exporting data from %d tables", len(tables))

	dataExporter := NewDataExporter(e.db, e.config)

	// Use worker pool for concurrent data export
	var wg sync.WaitGroup
	tableChan := make(chan string, len(tables))
	errChan := make(chan error, len(tables))

	// Start workers
	for i := 0; i < e.config.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for table := range tableChan {
				log.Printf("Worker %d: exporting table %s\n", workerID, table)
				if err := dataExporter.ExportTableData(table); err != nil {
					errChan <- fmt.Errorf("failed to export table %s: %v", table, err)
					return
				}
			}
		}(i)
	}

	// Send tables to workers
	for _, table := range tables {
		tableChan <- table
	}
	close(tableChan)

	// Wait for all workers
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

func (e *Exporter) getTables() ([]string, error) {
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = ? 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name
    `

	rows, err := e.db.Query(query, e.config.DatabaseName)
	if err != nil {
		return nil, err
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

	return tables, nil
}

func (e *Exporter) filterTables(tables []string) []string {
	if len(e.config.Tables) > 0 {
		// Include only specified tables
		tableMap := make(map[string]bool)
		for _, t := range e.config.Tables {
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

	if len(e.config.ExcludeTables) > 0 {
		// Exclude specified tables
		excludeMap := make(map[string]bool)
		for _, t := range e.config.ExcludeTables {
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

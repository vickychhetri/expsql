package importer

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

type Importer struct {
	db       *sql.DB
	inputDir string
	workers  int
}

func NewImporter(dsn, inputDir string, workers int) (*Importer, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	db.SetMaxOpenConns(workers * 2)
	db.SetMaxIdleConns(workers)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &Importer{
		db:       db,
		inputDir: inputDir,
		workers:  workers,
	}, nil
}

func (i *Importer) Close() error {
	return i.db.Close()
}

func (i *Importer) Import() error {
	// First import design files in correct order
	if err := i.importDesign(); err != nil {
		return fmt.Errorf("design import failed: %v", err)
	}

	// Then import data files concurrently
	if err := i.importData(); err != nil {
		return fmt.Errorf("data import failed: %v", err)
	}

	return nil
}

func (i *Importer) importDesign() error {
	// Import in order: tables, views, functions, procedures, triggers, events
	designFiles := []string{
		"design_tables.sql",
		"design_views.sql",
		"design_functions.sql",
		"design_procedures.sql",
		"design_triggers.sql",
		"design_events.sql",
	}

	for _, filename := range designFiles {
		filepath := filepath.Join(i.inputDir, filename)
		if err := i.executeSQLFile(filepath); err != nil {
			// Skip if file doesn't exist
			if strings.Contains(err.Error(), "no such file") {
				continue
			}
			return err
		}
		log.Printf("Imported: %s", filename)
	}

	return nil
}

func (i *Importer) importData() error {
	// Find all data files
	files, err := filepath.Glob(filepath.Join(i.inputDir, "data_*.sql"))
	if err != nil {
		return err
	}

	if len(files) == 0 {
		log.Println("No data files found")
		return nil
	}

	// Use worker pool for concurrent import
	var wg sync.WaitGroup
	fileChan := make(chan string, len(files))
	errChan := make(chan error, len(files))

	// Start workers
	for j := 0; j < i.workers; j++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for file := range fileChan {
				log.Printf("Worker %d: importing %s\n", workerID, filepath.Base(file))
				if err := i.executeSQLFile(file); err != nil {
					errChan <- fmt.Errorf("failed to import %s: %v", file, err)
					return
				}
			}
		}(j)
	}

	// Send files to workers
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

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

func (i *Importer) executeSQLFile(filepath string) error {
	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}

	// Split into individual statements
	statements := strings.Split(string(content), ";")

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		_, err := i.db.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute statement: %v\nStatement: %s", err, stmt)
		}
	}

	return nil
}

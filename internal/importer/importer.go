package importer

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

	i.db.Exec("SET FOREIGN_KEY_CHECKS=0")

	i.db.SetMaxOpenConns(1)
	if err := i.importDesign(); err != nil {
		return fmt.Errorf("design import failed: %v", err)
	}

	i.db.SetMaxOpenConns(i.workers * 2)
	if err := i.db.Ping(); err != nil {
		return err
	}

	i.db.Exec("FLUSH TABLES")

	if err := i.importData(); err != nil {
		return fmt.Errorf("data import failed: %v", err)
	}

	i.db.Exec("SET FOREIGN_KEY_CHECKS=1")

	return nil
}

func (i *Importer) importDesign() error {
	designFiles := []string{
		"design_tables.sql",
		"design_views.sql",
		"design_functions.sql",
		"design_procedures.sql",
		"design_triggers.sql",
		"design_events.sql",
	}

	for _, filename := range designFiles {
		fullPath := filepath.Join(i.inputDir, filename)

		// 🔍 Check file existence
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("SKIP: %s (file not found: %s)", filename, fullPath)
				continue
			}
			return fmt.Errorf("error checking file %s: %v", fullPath, err)
		}

		// 📊 Log file details
		log.Printf("FOUND: %s | Size: %d bytes | Path: %s",
			filename, info.Size(), fullPath)

		// 🚀 Execute
		if err := i.executeSQLFile(fullPath); err != nil {
			return fmt.Errorf("failed importing %s: %v", filename, err)
		}

		log.Printf("IMPORTED: %s", filename)
	}

	return nil
}

func (i *Importer) importData() error {
	// Find all data files
	files, err := filepath.Glob(filepath.Join(i.inputDir, "data_*.sql"))
	sort.Strings(files)
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

	var completed int64 = 0
	total := len(files)
	// Start workers
	for j := 0; j < i.workers; j++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for file := range fileChan {
				start := time.Now()

				log.Printf("[Worker %d] START: %s", workerID, filepath.Base(file))

				if err := i.executeSQLFile(file); err != nil {
					errChan <- fmt.Errorf("failed to import %s: %v", file, err)
					return
				}

				// increment safely
				newVal := atomic.AddInt64(&completed, 1)

				log.Printf(
					"[Worker %d] DONE: %s (%d/%d) in %s",
					workerID,
					filepath.Base(file),
					newVal,
					total,
					time.Since(start),
				)
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

	// 🔍 Print current DB
	var dbName string
	i.db.QueryRow("SELECT DATABASE()").Scan(&dbName)
	log.Println("CURRENT DB:", dbName)

	statements := splitSQLStatements(string(content))

	log.Printf("TOTAL STATEMENTS: %d\n", len(statements))

	for idx, stmt := range statements {
		stmt = cleanSQL(stmt)
		if stmt == "" {
			continue
		}

		upper := strings.ToUpper(stmt)

		// 🔥 Log important statements
		if strings.Contains(upper, "INSERT INTO") {
			log.Printf("\n--- EXEC [%d] ---\n%s\n----------------\n", idx, stmt[:40])
		}
		i.db.Exec("SET FOREIGN_KEY_CHECKS=0")
		_, err := i.db.Exec(stmt)
		i.db.Exec("SET FOREIGN_KEY_CHECKS=1")
		if err != nil {
			log.Printf("❌ FAILED [%d]: %v\n", idx, err)
			log.Printf("STATEMENT:\n%s\n", stmt)
			return fmt.Errorf("failed: %v\nstmt: %s", err, stmt)
		}
	}

	log.Println("✅ FILE EXECUTED:", filepath)

	// 🔍 Verify tables after execution
	rows, err := i.db.Query("SHOW TABLES")
	if err == nil {
		defer rows.Close()
		log.Println("📦 TABLES AFTER IMPORT:")
		var tbl string
		for rows.Next() {
			rows.Scan(&tbl)
			log.Println(" -", tbl)
		}
	}

	return nil
}

func splitSQLStatements(sqlText string) []string {
	var stmts []string
	var sb strings.Builder

	inSingle := false
	inDouble := false
	inBacktick := false
	inComment := false

	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]

		// Handle line comments --
		if !inSingle && !inDouble && !inBacktick {
			if i+1 < len(sqlText) && sqlText[i] == '-' && sqlText[i+1] == '-' {
				inComment = true
			}
			if inComment && ch == '\n' {
				inComment = false
			}
			if inComment {
				continue
			}
		}

		switch ch {
		case '\'':
			if !inDouble && !inBacktick {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		case ';':
			if !inSingle && !inDouble && !inBacktick && !inComment {
				stmt := strings.TrimSpace(sb.String())
				if stmt != "" {
					stmts = append(stmts, stmt)
				}
				sb.Reset()
				continue
			}
		}

		sb.WriteByte(ch)
	}

	if sb.Len() > 0 {
		stmt := strings.TrimSpace(sb.String())
		if stmt != "" {
			stmts = append(stmts, stmt)
		}
	}

	return stmts
}

func cleanSQL(stmt string) string {
	stmt = strings.TrimSpace(stmt)

	if stmt == "" {
		return ""
	}

	// skip only pure comments
	if strings.HasPrefix(stmt, "--") {
		return ""
	}

	return stmt
}

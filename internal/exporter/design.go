package exporter

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

type DesignExporter struct {
	db           *sql.DB
	outputDir    string
	compress     bool
	databaseName string
}

func NewDesignExporter(db *sql.DB, outputDir string, compress bool, databaseName string) *DesignExporter {
	return &DesignExporter{
		db:           db,
		outputDir:    outputDir,
		compress:     compress,
		databaseName: databaseName,
	}
}

func (d *DesignExporter) ExportTables(tables []string) error {
	file, err := d.createFile("design_tables.sql")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.WriteString(fmt.Sprintf("-- Database: %s\n", d.databaseName))
	writer.WriteString("-- ============================================\n")
	writer.WriteString("-- TABLE STRUCTURES\n")
	writer.WriteString("-- ============================================\n\n")

	// Disable foreign key checks for safe import
	writer.WriteString("SET FOREIGN_KEY_CHECKS=0;\n\n")

	for _, table := range tables {
		// Get CREATE TABLE statement
		var tableName, createTable string
		query := fmt.Sprintf("SHOW CREATE TABLE `%s`", table)
		err := d.db.QueryRow(query).Scan(&tableName, &createTable)
		if err != nil {
			return fmt.Errorf("failed to get create table for %s: %v", table, err)
		}

		writer.WriteString(fmt.Sprintf("-- Table: %s\n", table))
		writer.WriteString(createTable)
		writer.WriteString(";\n\n")

		// Get table indexes
		indexes, err := d.getTableIndexes(table)
		if err == nil && len(indexes) > 0 {
			writer.WriteString(fmt.Sprintf("-- Indexes for %s\n", table))
			for _, index := range indexes {
				writer.WriteString(index)
				writer.WriteString(";\n")
			}
			writer.WriteString("\n")
		}
	}

	// Re-enable foreign key checks
	writer.WriteString("SET FOREIGN_KEY_CHECKS=1;\n")

	return nil
}

func (d *DesignExporter) ExportViews() error {
	views, err := d.getViews()
	if err != nil {
		return err
	}

	if len(views) == 0 {
		return nil
	}

	file, err := d.createFile("design_views.sql")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	writer.WriteString(fmt.Sprintf("-- Database: %s\n", d.databaseName))
	writer.WriteString("-- ============================================\n")
	writer.WriteString("-- VIEWS\n")
	writer.WriteString("-- ============================================\n\n")

	for _, view := range views {
		var viewName, createView string
		query := fmt.Sprintf("SHOW CREATE VIEW `%s`", view)
		err := d.db.QueryRow(query).Scan(&viewName, &createView)
		if err != nil {
			return fmt.Errorf("failed to get create view for %s: %v", view, err)
		}

		writer.WriteString(fmt.Sprintf("-- View: %s\n", view))
		writer.WriteString(createView)
		writer.WriteString(";\n\n")
	}

	return nil
}

func (d *DesignExporter) ExportFunctions() error {
	functions, err := d.getRoutines("FUNCTION")
	if err != nil {
		return err
	}

	if len(functions) == 0 {
		return nil
	}

	file, err := d.createFile("design_functions.sql")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	writer.WriteString(fmt.Sprintf("-- Database: %s\n", d.databaseName))
	writer.WriteString("-- ============================================\n")
	writer.WriteString("-- FUNCTIONS\n")
	writer.WriteString("-- ============================================\n\n")

	for _, fn := range functions {
		createSQL, err := d.getRoutineSQL(fn, "FUNCTION")
		if err != nil {
			return err
		}

		writer.WriteString(fmt.Sprintf("-- Function: %s\n", fn))
		writer.WriteString(createSQL)
		writer.WriteString(";\n\n")
	}

	return nil
}

func (d *DesignExporter) ExportProcedures() error {
	procedures, err := d.getRoutines("PROCEDURE")
	if err != nil {
		return err
	}

	if len(procedures) == 0 {
		return nil
	}

	file, err := d.createFile("design_procedures.sql")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	writer.WriteString(fmt.Sprintf("-- Database: %s\n", d.databaseName))
	writer.WriteString("-- ============================================\n")
	writer.WriteString("-- STORED PROCEDURES\n")
	writer.WriteString("-- ============================================\n\n")

	for _, proc := range procedures {
		createSQL, err := d.getRoutineSQL(proc, "PROCEDURE")
		if err != nil {
			return err
		}

		writer.WriteString(fmt.Sprintf("-- Procedure: %s\n", proc))
		writer.WriteString(createSQL)
		writer.WriteString(";\n\n")
	}

	return nil
}

func (d *DesignExporter) ExportTriggers() error {
	triggers, err := d.getTriggers()
	if err != nil {
		return err
	}

	if len(triggers) == 0 {
		return nil
	}

	file, err := d.createFile("design_triggers.sql")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	writer.WriteString(fmt.Sprintf("-- Database: %s\n", d.databaseName))
	writer.WriteString("-- ============================================\n")
	writer.WriteString("-- TRIGGERS\n")
	writer.WriteString("-- ============================================\n\n")

	for _, trigger := range triggers {
		var triggerName, createTrigger string
		query := fmt.Sprintf("SHOW CREATE TRIGGER `%s`", trigger)
		err := d.db.QueryRow(query).Scan(&triggerName, &createTrigger)
		if err != nil {
			return fmt.Errorf("failed to get create trigger for %s: %v", trigger, err)
		}

		writer.WriteString(fmt.Sprintf("-- Trigger: %s\n", trigger))
		writer.WriteString(createTrigger)
		writer.WriteString(";\n\n")
	}

	return nil
}

func (d *DesignExporter) ExportEvents() error {
	events, err := d.getEvents()
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	file, err := d.createFile("design_events.sql")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	writer.WriteString(fmt.Sprintf("-- Database: %s\n", d.databaseName))
	writer.WriteString("-- ============================================\n")
	writer.WriteString("-- EVENTS\n")
	writer.WriteString("-- ============================================\n\n")

	for _, event := range events {
		var eventName, createEvent string
		query := fmt.Sprintf("SHOW CREATE EVENT `%s`", event)
		err := d.db.QueryRow(query).Scan(&eventName, &createEvent)
		if err != nil {
			return fmt.Errorf("failed to get create event for %s: %v", event, err)
		}

		writer.WriteString(fmt.Sprintf("-- Event: %s\n", event))
		writer.WriteString(createEvent)
		writer.WriteString(";\n\n")
	}

	return nil
}

func (d *DesignExporter) createFile(filename string) (*os.File, error) {
	filepath := filepath.Join(d.outputDir, filename)
	return os.Create(filepath)
}

func (d *DesignExporter) getTableIndexes(table string) ([]string, error) {
	query := `
        SELECT index_name, non_unique, GROUP_CONCAT(column_name ORDER BY seq_in_index) as columns
        FROM information_schema.statistics 
        WHERE table_schema = ? 
        AND table_name = ?
        AND index_name != 'PRIMARY'
        GROUP BY index_name, non_unique
        ORDER BY index_name
    `

	rows, err := d.db.Query(query, d.databaseName, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var indexName string
		var nonUnique int
		var columns string

		err := rows.Scan(&indexName, &nonUnique, &columns)
		if err != nil {
			return nil, err
		}

		unique := ""
		if nonUnique == 0 {
			unique = "UNIQUE "
		}

		createIndex := fmt.Sprintf("CREATE %sINDEX `%s` ON `%s` (%s)",
			unique, indexName, table, columns)
		result = append(result, createIndex)
	}

	return result, nil
}

func (d *DesignExporter) getViews() ([]string, error) {
	query := `
        SELECT table_name 
        FROM information_schema.views 
        WHERE table_schema = ?
        ORDER BY table_name
    `

	rows, err := d.db.Query(query, d.databaseName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []string
	for rows.Next() {
		var view string
		if err := rows.Scan(&view); err != nil {
			return nil, err
		}
		views = append(views, view)
	}

	return views, nil
}

func (d *DesignExporter) getRoutines(routineType string) ([]string, error) {
	query := `
        SELECT routine_name 
        FROM information_schema.routines 
        WHERE routine_schema = ? 
        AND routine_type = ?
        ORDER BY routine_name
    `

	rows, err := d.db.Query(query, d.databaseName, routineType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routines []string
	for rows.Next() {
		var routine string
		if err := rows.Scan(&routine); err != nil {
			return nil, err
		}
		routines = append(routines, routine)
	}

	return routines, nil
}

func (d *DesignExporter) getRoutineSQL(name, routineType string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE %s `%s`", routineType, name)
	var routineName, createSQL string
	err := d.db.QueryRow(query).Scan(&routineName, &createSQL)
	if err != nil {
		return "", err
	}
	return createSQL, nil
}

func (d *DesignExporter) getTriggers() ([]string, error) {
	query := `
        SELECT trigger_name 
        FROM information_schema.triggers 
        WHERE trigger_schema = ?
        ORDER BY trigger_name
    `

	rows, err := d.db.Query(query, d.databaseName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []string
	for rows.Next() {
		var trigger string
		if err := rows.Scan(&trigger); err != nil {
			return nil, err
		}
		triggers = append(triggers, trigger)
	}

	return triggers, nil
}

func (d *DesignExporter) getEvents() ([]string, error) {
	query := `
        SELECT event_name 
        FROM information_schema.events 
        WHERE event_schema = ?
        ORDER BY event_name
    `

	rows, err := d.db.Query(query, d.databaseName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []string
	for rows.Next() {
		var event string
		if err := rows.Scan(&event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

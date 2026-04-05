package cmd

import (
	"expsql/internal/exporter"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	exportDir           string
	workers             int
	rowsPerBatch        int
	compress            bool
	includeData         bool
	includeDesign       bool
	tables              []string
	excludeTables       []string
	bulkInsertSize      int
	chunkByPK           bool
	noLock              bool
	smallTableThreshold int
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export MySQL database",
	Long: `Export MySQL database schema and data to files.
The export creates separate files for:
- Design: DDL statements for tables, views, functions, procedures, triggers, events
- Data: Data dumps for each table with concurrent export`,
	Run: func(cmd *cobra.Command, args []string) {
		startTime := time.Now()
		// Validate flags
		if workers <= 0 {
			log.Fatal("Workers must be greater than 0")
		}

		if bulkInsertSize <= 0 {
			log.Fatal("Bulk insert size must be greater than 0")
		}

		if bulkInsertSize > rowsPerBatch {
			log.Println("⚠️ bulk-size > rows-per-batch, adjusting to rows-per-batch")
			bulkInsertSize = rowsPerBatch
		}

		if rowsPerBatch <= 0 {
			log.Fatal("Rows per batch must be greater than 0")
		}

		// Create export directory
		if err := os.MkdirAll(exportDir, 0755); err != nil {
			log.Fatalf("Failed to create export directory: %v", err)
		}

		// Build database connection string
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
			dbUser, dbPass, dbHost, dbPort, dbName)

		// Create exporter
		exp, err := exporter.NewExporter(dsn, &exporter.ExporterConfig{
			OutputDir:     exportDir,
			Workers:       workers,
			RowsPerBatch:  rowsPerBatch,
			Compress:      compress,
			IncludeData:   includeData,
			IncludeDesign: includeDesign,
			Tables:        tables,
			ExcludeTables: excludeTables,
		})
		if err != nil {
			log.Fatalf("Failed to create exporter: %v", err)
		}
		defer exp.Close()

		// Run export
		if err := exp.Export(); err != nil {
			log.Fatalf("Export failed: %v", err)
		}

		endTime := time.Now()
		duration := endTime.Sub(startTime)

		log.Printf("\n" + strings.Repeat("=", 50))
		log.Printf("✅ Export completed successfully!")
		log.Printf("⏱️  Total time: %s", duration)
		log.Printf("📅 Started: %s", startTime.Format("2006-01-02 15:04:05"))
		log.Printf("🏁 Completed: %s", endTime.Format("2006-01-02 15:04:05"))
		log.Printf(strings.Repeat("=", 50))
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVar(&exportDir, "output", "./export", "Output directory")
	exportCmd.Flags().IntVar(&workers, "workers", 4, "Number of concurrent workers for data export")
	exportCmd.Flags().IntVar(&rowsPerBatch, "rows-per-batch", 10000, "Number of rows per batch for data export")
	exportCmd.Flags().IntVar(&bulkInsertSize, "bulk-size", 1000, "Rows per bulk INSERT")
	exportCmd.Flags().BoolVar(&compress, "compress", false, "Compress output files")
	exportCmd.Flags().BoolVar(&includeData, "include-data", true, "Include table data in export")
	exportCmd.Flags().BoolVar(&includeDesign, "include-design", true, "Include database design in export")
	exportCmd.Flags().StringSliceVar(&tables, "tables", []string{}, "Specific tables to export (comma-separated)")
	exportCmd.Flags().StringSliceVar(&excludeTables, "exclude-tables", []string{}, "Tables to exclude from export")
	exportCmd.Flags().BoolVar(&noLock, "no-lock", false, "Disable table locking (for live DB)")
	exportCmd.Flags().BoolVar(&chunkByPK, "chunk-by-pk", false, "Use primary key chunking instead of OFFSET")
}

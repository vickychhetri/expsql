package cmd

import (
	"expsql/internal/exporter"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	parallelExportDir     string
	parallelWorkers       int
	parallelRowsPerBatch  int
	parallelCompress      bool
	parallelIncludeData   bool
	parallelIncludeDesign bool
	parallelTables        []string
	parallelExcludeTables []string
	parallelStrategy      string // auto, parallel, streaming, standard
	parallelPartitions    int    // number of partitions for parallel export
	parallelResumable     bool   // enable resumable export
	parallelProgressDir   string // directory for progress files
)

var exportParallelCmd = &cobra.Command{
	Use:   "export-parallel",
	Short: "Export MySQL database with advanced parallel processing",
	Long: `Export MySQL database schema and data with high-performance parallel processing.
This command provides multiple export strategies optimized for different scenarios:

Strategies:
  - auto: Automatically choose best strategy based on table size
  - parallel: Use parallel partitioning for very large tables (>10M rows)
  - streaming: Use streaming approach for medium-large tables (5-10M rows)
  - standard: Use standard batch export for small tables (<5M rows)

Features:
  - Parallel table partitioning for ultra-fast exports
  - Streaming export for memory-efficient processing
  - Resumable exports for very large datasets
  - Progress tracking with ETA
  - Automatic strategy selection based on table size
  - Separate design and data files`,
	Run: func(cmd *cobra.Command, args []string) {
		// Validate flags
		if parallelWorkers <= 0 {
			log.Fatal("Workers must be greater than 0")
		}
		if parallelRowsPerBatch <= 0 {
			log.Fatal("Rows per batch must be greater than 0")
		}
		if parallelPartitions <= 0 {
			parallelPartitions = parallelWorkers
		}

		// Create export directory
		if err := os.MkdirAll(parallelExportDir, 0755); err != nil {
			log.Fatalf("Failed to create export directory: %v", err)
		}

		// Create progress directory if resumable
		if parallelResumable {
			if parallelProgressDir == "" {
				parallelProgressDir = filepath.Join(parallelExportDir, ".progress")
			}
			if err := os.MkdirAll(parallelProgressDir, 0755); err != nil {
				log.Fatalf("Failed to create progress directory: %v", err)
			}
		}

		// encPassword := url.QueryEscape(dbPass)

		// Build database connection string
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
			dbUser, dbPass, dbHost, dbPort, dbName)

		// Create exporter configuration
		config := &exporter.ExporterConfig{
			OutputDir:     parallelExportDir,
			Workers:       parallelWorkers,
			RowsPerBatch:  parallelRowsPerBatch,
			Compress:      parallelCompress,
			IncludeData:   parallelIncludeData,
			IncludeDesign: parallelIncludeDesign,
			Tables:        parallelTables,
			ExcludeTables: parallelExcludeTables,
		}

		// Create advanced exporter
		advExporter, err := exporter.NewAdvancedExporter(dsn, config, &exporter.AdvancedConfig{
			Strategy:    parallelStrategy,
			Partitions:  parallelPartitions,
			Resumable:   parallelResumable,
			ProgressDir: parallelProgressDir,
		})
		if err != nil {
			log.Fatalf("Failed to create advanced exporter: %v", err)
		}
		defer advExporter.Close()

		// Run export
		if err := advExporter.Export(); err != nil {
			log.Fatalf("Export failed: %v", err)
		}

		log.Println("Export completed successfully!")
	},
}

func init() {
	rootCmd.AddCommand(exportParallelCmd)

	// Basic flags
	exportParallelCmd.Flags().StringVar(&parallelExportDir, "output", "./export", "Output directory")
	exportParallelCmd.Flags().IntVar(&parallelWorkers, "workers", 4, "Number of concurrent workers")
	exportParallelCmd.Flags().IntVar(&parallelRowsPerBatch, "rows-per-batch", 10000, "Number of rows per batch")
	exportParallelCmd.Flags().BoolVar(&parallelCompress, "compress", false, "Compress output files")
	exportParallelCmd.Flags().BoolVar(&parallelIncludeData, "include-data", true, "Include table data")
	exportParallelCmd.Flags().BoolVar(&parallelIncludeDesign, "include-design", true, "Include database design")
	exportParallelCmd.Flags().StringSliceVar(&parallelTables, "tables", []string{}, "Specific tables to export")
	exportParallelCmd.Flags().StringSliceVar(&parallelExcludeTables, "exclude-tables", []string{}, "Tables to exclude")

	// Advanced flags
	exportParallelCmd.Flags().StringVar(&parallelStrategy, "strategy", "auto",
		"Export strategy: auto, parallel, streaming, standard")
	exportParallelCmd.Flags().IntVar(&parallelPartitions, "partitions", 0,
		"Number of partitions for parallel export (default: workers count)")
	exportParallelCmd.Flags().BoolVar(&parallelResumable, "resumable", false,
		"Enable resumable export (saves progress)")
	exportParallelCmd.Flags().StringVar(&parallelProgressDir, "progress-dir", "",
		"Directory for progress files (default: output/.progress)")
}

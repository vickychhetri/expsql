package cmd

import (
	"expsql/internal/importer"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var (
	importDir     string
	importWorkers int
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import MySQL database from exported files",
	Long: `Import MySQL database from previously exported files.
The import process handles both design and data files in the correct order.`,
	Run: func(cmd *cobra.Command, args []string) {
		if importWorkers <= 0 {
			log.Fatal("Workers must be greater than 0")
		}

		// Build database connection string
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
			dbUser, dbPass, dbHost, dbPort, dbName)

		// Create importer
		imp, err := importer.NewImporter(dsn, importDir, importWorkers)
		if err != nil {
			log.Fatalf("Failed to create importer: %v", err)
		}
		defer imp.Close()

		// Run import
		if err := imp.Import(); err != nil {
			log.Fatalf("Import failed: %v", err)
		}

		log.Println("Import completed successfully!")
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVar(&importDir, "input", "./export", "Input directory containing exported files")
	importCmd.Flags().IntVar(&importWorkers, "workers", 4, "Number of concurrent workers for data import")
}

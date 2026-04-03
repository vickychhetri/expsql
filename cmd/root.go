package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	dbHost  string
	dbPort  int
	dbUser  string
	dbPass  string
	dbName  string
)

var rootCmd = &cobra.Command{
	Use:   "mysqltool",
	Short: "A high-performance MySQL database export/import tool",
	Long: `MySQL Tool is a high-performance utility for exporting and importing 
MySQL databases with support for large datasets (50GB+). 
It uses concurrent workers for fast data export and handles 
both schema (design) and data separately.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mysqltool.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbHost, "host", "localhost", "MySQL host")
	rootCmd.PersistentFlags().IntVar(&dbPort, "port", 3306, "MySQL port")
	rootCmd.PersistentFlags().StringVar(&dbUser, "user", "root", "MySQL user")
	rootCmd.PersistentFlags().StringVar(&dbPass, "password", "", "MySQL password")
	rootCmd.PersistentFlags().StringVar(&dbName, "database", "", "Database name (required)")

	rootCmd.MarkPersistentFlagRequired("database")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName(".mysqltool")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME")
		viper.AddConfigPath(".")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

package cmd

import (
	"github.com/spf13/cobra"
)

var apiURL string

var rootCmd = &cobra.Command{
	Use:   "enverge",
	Short: "Enverge CLI",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "API base URL")
}

func Execute() error {
	return rootCmd.Execute()
}

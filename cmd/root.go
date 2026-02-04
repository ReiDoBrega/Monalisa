package cmd

import (
	"monalisa/pkg/logger"

	"github.com/spf13/cobra"
)

var (
	rootCmd   *cobra.Command
	verbose   bool
	noColor   bool
	timestamp bool
)

func Execute(version string) error {
	rootCmd = &cobra.Command{
		Use:   "monalisa",
		Short: "MonaLisa Content Decryption Module for Go",
		Long:  "A Go library to decrypt IQIYI DRM License Ticket",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Configurar logger baseado nas flags
			if verbose {
				logger.SetLevel(logger.DEBUG)
			}
			logger.SetColored(!noColor)
			logger.SetTimestamp(timestamp)
		},
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info("monalisa version %s", version)
			logger.Info("MonaLisa Content Decryption Module for Go")
			logger.Info("Run 'monalisa --help' for help")
		},
	}

	// Flags globais
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&timestamp, "timestamp", false, "Add timestamp to log messages")

	rootCmd.AddCommand(licenseCmd)
	rootCmd.AddCommand(createDeviceCmd)
	rootCmd.AddCommand(verifyDeviceCmd)
	rootCmd.AddCommand(licenseInfoCmd)

	return rootCmd.Execute()
}

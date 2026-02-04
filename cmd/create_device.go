package cmd

import (
	"monalisa/pkg/logger"
	"monalisa/pkg/module"

	"github.com/spf13/cobra"
)

var (
	watPath       string
	wasmPath      string
	jsPath        string
	outputPath    string
	noCompress    bool
	moduleVersion string
	moduleName    string
	moduleDesc    string
	validDays     int
)

var createDeviceCmd = &cobra.Command{
	Use:   "create-device",
	Short: "Build a self-contained binary .mld device file",
	Long: `Build a self-contained binary .mld device file.

Examples:
  # From WASM binary:
  monalisa create-device --wasm module.wasm -o device.mld

  # From WAT source with JS wrapper:
  monalisa create-device --wat module.wat --js wrapper.js -o device.mld

  # With custom metadata and 30-day license:
  monalisa create-device --wasm module.wasm -o device.mld \
    --version "4.0.0" --name "Custom Module" --valid-days 30

  # Permanent license (no expiration):
  monalisa create-device --wasm module.wasm -o device.mld --valid-days 0`,
	Run: runCreateDevice,
}

func init() {
	createDeviceCmd.Flags().StringVar(&watPath, "wat", "", "Input WAT (WebAssembly Text) file")
	createDeviceCmd.Flags().StringVar(&wasmPath, "wasm", "", "Input WASM (WebAssembly Binary) file")
	createDeviceCmd.Flags().StringVar(&jsPath, "js", "", "JavaScript wrapper file (optional)")
	createDeviceCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output .mld device file path (required)")
	createDeviceCmd.Flags().BoolVar(&noCompress, "no-compress", false, "Disable compression")
	createDeviceCmd.Flags().StringVar(&moduleVersion, "version", "3.0.6", "Module version")
	createDeviceCmd.Flags().StringVar(&moduleName, "name", "MonaLisa Browser Module", "Module name")
	createDeviceCmd.Flags().StringVar(&moduleDesc, "description", "", "Module description")
	createDeviceCmd.Flags().IntVar(&validDays, "valid-days", 30, "License validity in days (0 = no expiration)")

	createDeviceCmd.MarkFlagRequired("output")
}

func runCreateDevice(cmd *cobra.Command, args []string) {
	if watPath == "" && wasmPath == "" {
		logger.Fatal("Must specify either --wat or --wasm")
	}

	builder := module.NewBuilder()

	logger.Info("Configuring module metadata")
	builder.WithMetadata(moduleVersion, moduleName, moduleDesc)
	logger.Debug("  Module: %s v%s", moduleName, moduleVersion)
	if moduleDesc != "" {
		logger.Debug("  Description: %s", moduleDesc)
	}

	if watPath != "" {
		logger.Info("Loading WAT source: %s", watPath)
		if err := builder.WithWAT(watPath); err != nil {
			logger.Fatal("Failed to load WAT: %v", err)
		}
	} else if wasmPath != "" {
		logger.Info("Loading WASM binary: %s", wasmPath)
		if err := builder.WithWASM(wasmPath); err != nil {
			logger.Fatal("Failed to load WASM: %v", err)
		}
	}

	if jsPath != "" {
		logger.Info("Loading JavaScript wrapper: %s", jsPath)
		if err := builder.WithJS(jsPath); err != nil {
			logger.Warning("Failed to load JS: %v", err)
		}
	}

	builder.WithCompression(!noCompress)
	if noCompress {
		logger.Info("Compression disabled")
	} else {
		logger.Info("Compression enabled (level 9)")
	}

	// Configura a licença
	if validDays > 0 {
		logger.Info("Configuring license: %d days validity", validDays)
		builder.WithLicense(validDays)
	} else {
		logger.Info("No license expiration (permanent)")
	}

	logger.Info("Building device file...")
	if err := builder.Build(outputPath); err != nil {
		logger.Fatal("Build failed: %v", err)
	}

	logger.Success("Device file created successfully: %s", outputPath)
	if validDays > 0 {
		logger.Warning("This device will expire in %d days", validDays)
	}
	logger.Info("Use: monalisa license --device %s <license_data>", outputPath)
}

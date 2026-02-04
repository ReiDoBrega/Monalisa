package cmd

import (
	"encoding/hex"
	"encoding/json"
	"os"

	"monalisa/pkg/cdm"
	"monalisa/pkg/license"
	"monalisa/pkg/logger"
	"monalisa/pkg/module"
	"monalisa/pkg/types"

	"github.com/spf13/cobra"
)

var (
	devicePath string
	keyType    string
	jsonOutput string
	quiet      bool
)

type KeyOutput struct {
	KID  string `json:"kid"`
	Key  string `json:"key"`
	Type string `json:"type"`
}

type LicenseOutput struct {
	Success bool        `json:"success"`
	Keys    []KeyOutput `json:"keys"`
	Count   int         `json:"count"`
}

var licenseCmd = &cobra.Command{
	Use:   "license LICENSE_DATA",
	Short: "Process a MonaLisa encoded license and extract decryption keys",
	Long: `Process a MonaLisa encoded license and extract decryption keys.

Examples:
  # Output to console
  monalisa license "AIUACgMAAAAAAAAAAAQChgACATADhwAnAgAg..." --device device.mld  

  # Output to JSON file
  monalisa license "AIUACgMAAAAAAAAAAAQChgACATADhwAnAgAg..." --device device.mld --json keys.json 

  # Quiet mode (only output keys)
  monalisa license "AIUACgMAAAAAAAAAAAQChgACATADhwAnAgAg..." --device device.mld --json keys.json --quiet `,
	Args: cobra.ExactArgs(1),
	Run:  runLicense,
}

func init() {
	licenseCmd.Flags().StringVarP(&devicePath, "device", "d", "", "Path to MonaLisa device file (.mld) (required)")
	licenseCmd.Flags().StringVarP(&keyType, "key-type", "t", "CONTENT", "Filter keys by type (CONTENT, FULL)")
	licenseCmd.Flags().StringVarP(&jsonOutput, "json", "j", "", "Output keys to JSON file")
	licenseCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode - only output keys")
	licenseCmd.MarkFlagRequired("device")
}

func runLicense(cmd *cobra.Command, args []string) {
	licenseData := args[0]

	// Se quiet mode, desabilitar todos os logs exceto erros
	if quiet {
		logger.SetLevel(logger.ERROR)
	}

	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		logger.Fatal("Module file not found: %s", devicePath)
	}

	logger.Info("Loading module: %s", devicePath)
	mod, err := module.Load(devicePath)
	if err != nil {
		logger.Fatal("Failed to load module: %v", err)
	}
	logger.Success("Loaded module successfully")

	logger.Info("Initializing CDM...")
	cdmInstance := cdm.FromModule(mod)

	logger.Info("Opening CDM session...")
	sessionID, err := cdmInstance.Open()
	if err != nil {
		logger.Fatal("Failed to open session: %v", err)
	}
	logger.Debug("Session opened: %s", sessionID)
	defer cdmInstance.Close(sessionID)

	logger.Info("Processing license data...")
	lic := license.New(licenseData)

	logger.Info("Parsing license and extracting keys...")
	if err := cdmInstance.ParseLicense(sessionID, lic); err != nil {
		logger.Fatal("Failed to parse license: %v", err)
	}
	logger.Success("License parsed successfully")

	keys, err := cdmInstance.GetKeys(sessionID, types.KeyTypeContent)
	if err != nil {
		logger.Fatal("Failed to get keys: %v", err)
	}

	if len(keys) == 0 {
		logger.Warning("No keys found in license")
		os.Exit(1)
	}

	// Se JSON output foi especificado
	if jsonOutput != "" {
		outputKeys := make([]KeyOutput, len(keys))
		for i, key := range keys {
			outputKeys[i] = KeyOutput{
				KID:  hex.EncodeToString(key.KID),
				Key:  hex.EncodeToString(key.Key),
				Type: key.Type.String(),
			}
		}

		output := LicenseOutput{
			Success: true,
			Keys:    outputKeys,
			Count:   len(keys),
		}

		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			logger.Fatal("Failed to marshal JSON: %v", err)
		}

		if err := os.WriteFile(jsonOutput, jsonData, 0644); err != nil {
			logger.Fatal("Failed to write JSON file: %v", err)
		}

		logger.Success("Keys saved to: %s", jsonOutput)
	}

	// Output para console
	logger.Info("Found %d key(s):", len(keys))

	// Se quiet mode, apenas imprime as chaves sem logger
	if quiet {
		for _, key := range keys {
			if keyType == "CONTENT" {
				logger.Info("%x", key.Key)
			} else {
				logger.Info("%x:%x", key.KID, key.Key)
			}
		}
	} else {
		for _, key := range keys {
			if keyType == "CONTENT" {
				logger.Info("%x", key.Key)
			} else {
				logger.Info("%x:%x", key.KID, key.Key)
			}
		}
	}

	logger.Success("Session closed successfully")
}

package cmd

import (
	"fmt"
	"os"
	"time"

	"monalisa/pkg/logger"
	"monalisa/pkg/module"

	"github.com/spf13/cobra"
)

var licenseInfoCmd = &cobra.Command{
	Use:   "license-info DEVICE_PATH",
	Short: "Display license information for a device file",
	Long: `Display license information for a device file including expiry date and validity status.

Example:
  monalisa license-info device.mld`,
	Args: cobra.ExactArgs(1),
	Run:  runLicenseInfo,
}

func runLicenseInfo(cmd *cobra.Command, args []string) {
	devicePath := args[0]

	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		logger.Fatal("Device file not found: %s", devicePath)
	}

	logger.Info("Loading device: %s", devicePath)
	mod, err := module.Load(devicePath)
	if err != nil {
		logger.Fatal("Failed to load device: %v", err)
	}

	sig := mod.Signature()

	separator := "//"
	fmt.Println("\n" + separator)
	fmt.Println("+ DEVICE LICENSE INFORMATION")
	fmt.Println(separator)

	if sig == nil {
		logger.Success("License: PERMANENT (No expiration)")
		fmt.Println("  This device has no time restrictions")
	} else {
		fmt.Printf("\n  Device ID: %s\n", sig.DeviceID)
		fmt.Printf("  Created: %s\n", sig.CreationTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Expires: %s\n", sig.ExpiryTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Max Duration: %d days\n", sig.MaxValidDuration/(24*60*60))

		now := time.Now()

		// Verifica adulteração de relógio
		if now.Before(sig.CreationTime) {
			daysBackwards := int(sig.CreationTime.Sub(now).Hours() / 24)
			fmt.Printf("\n")
			logger.Warning("CLOCK TAMPERING DETECTED!")
			fmt.Printf("  System time is %d day(s) BEFORE device creation\n", daysBackwards)
			fmt.Printf("  Device cannot be used until system clock is corrected\n")
		} else if now.After(sig.ExpiryTime) {
			daysExpired := int(now.Sub(sig.ExpiryTime).Hours() / 24)
			fmt.Printf("\n")
			logger.Failure("Status: EXPIRED")
			fmt.Printf("  Expired %d day(s) ago\n", daysExpired)
		} else {
			// Verifica se excedeu a duração máxima
			elapsedTime := now.Unix() - sig.GetCreationTimestamp()
			if elapsedTime > sig.MaxValidDuration {
				fmt.Printf("\n")
				logger.Failure("Status: MAXIMUM DURATION EXCEEDED")
				fmt.Printf("  Time since creation: %d days (max: %d days)\n",
					elapsedTime/(24*60*60),
					sig.MaxValidDuration/(24*60*60))
				logger.Warning("Possible clock tampering detected")
			} else {
				daysRemaining := int(sig.ExpiryTime.Sub(now).Hours() / 24)
				hoursRemaining := int(sig.ExpiryTime.Sub(now).Hours()) % 24
				daysSinceCreation := int(now.Sub(sig.CreationTime).Hours() / 24)

				fmt.Printf("\n")
				logger.Success("Status: VALID")
				fmt.Printf("  Days since creation: %d day(s)\n", daysSinceCreation)
				fmt.Printf("  Time remaining: %d day(s) and %d hour(s)\n", daysRemaining, hoursRemaining)

				if daysRemaining <= 7 {
					logger.Warning("License expiring soon!")
				}
			}
		}
	}

	fmt.Println("\n" + separator)
}

func init() {
	// Esta função será chamada pelo root.go!
}

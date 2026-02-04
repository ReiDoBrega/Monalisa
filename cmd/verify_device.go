package cmd

import (
	"monalisa/pkg/logger"
	"monalisa/pkg/module"

	"github.com/spf13/cobra"
)

var verifyDeviceCmd = &cobra.Command{
	Use:   "verify-device DEVICE_PATH",
	Short: "Verify the integrity of an .mld device file",
	Long: `Verify the integrity of an .mld device file.

Example:
  monalisa verify-device device.mld`,
	Args: cobra.ExactArgs(1),
	Run:  runVerifyDevice,
}

func runVerifyDevice(cmd *cobra.Command, args []string) {
	devicePath := args[0]

	logger.Info("Verifying device: %s", devicePath)

	builder := module.NewBuilder()
	if err := builder.Verify(devicePath); err != nil {
		logger.Failure("Verification failed: %v", err)
		return
	}

	logger.Success("Device file verification PASSED")
}

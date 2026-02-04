package main

import (
	"monalisa/cmd"
	"monalisa/pkg/logger"
)

const version = "0.1.2"

func main() {
	logger.Init(logger.INFO, true)

	if err := cmd.Execute(version); err != nil {
		logger.Fatal("Application error: %v", err)
	}
}

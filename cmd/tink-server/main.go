package main

import (
	"os"

	"github.com/packethost/pkg/log"
	"github.com/tinkerbell/tink/cmd/tink-server/cmd"
)

const (
	serviceKey = "github.com/tinkerbell/tink"
)

// version is set at build time.
var version = "devel"

func main() {
	retCode := 0

	defer func() { os.Exit(retCode) }()

	logger, err := log.Init(serviceKey)
	if err != nil {
		panic(err)
	}

	defer logger.Close()

	rootCmd := cmd.NewRootCommand(version, logger)
	if err := rootCmd.Execute(); err != nil {
		retCode = 1

		return
	}
}

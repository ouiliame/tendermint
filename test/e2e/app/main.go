package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/libs/log"
)

var logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %v <configfile>", os.Args[0])
		return
	}
	configFile := ""
	if len(os.Args) == 2 {
		configFile = os.Args[1]
	}

	if err := run(configFile); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func run(configFile string) error {
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	app, err := NewApplication(cfg)
	if err != nil {
		return err
	}

	protocol := "socket"
	if cfg.GRPC {
		protocol = "grpc"
	}
	server, err := server.NewServer(cfg.Listen, protocol, app)
	if err != nil {
		return err
	}
	err = server.Start()
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("Server listening on %v (%v protocol)", cfg.Listen, protocol))

	// Apparently there's no way to wait for the server, so we just sleep
	for {
		time.Sleep(1 * time.Hour)
	}
}

package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func trapSigInt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-c
}

func main() {
	slog.Info("Starting")

	trapSigInt()

	slog.Info("Stopping")
}

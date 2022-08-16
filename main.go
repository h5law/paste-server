package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/h5law/paste-server/api"
	"github.com/h5law/paste-server/cmd"
)

func main() {
	// Get command line arguments
	cmd.Execute()

	// Enable graceful shutdown on signal interrupts
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	// Listen for interrupt
	go func() {
		oscall := <-c
		log.Printf("system call: %v\n", oscall)
		cancel()
	}()

	if err := api.StartServer(ctx); err != nil {
		log.Fatalf("failed to start server: %v\n", err)
	}
}

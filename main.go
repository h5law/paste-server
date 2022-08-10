package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/h5law/paste-server/api"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// Load environment varaibles
func goDotEnvVariable(key string) string {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Error loading .env file")
	}
	return os.Getenv(key)
}

var appEnv string
var mdbUri string

var Port int

var rootCmd = &cobra.Command{
	Use:   "paste-server",
	Short: "Start a paste-server instance locally",
	Long:  `Start a paste-server instance to allow for CRUD operations for ephemeral pastes on a given port`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func main() {
	appEnv = goDotEnvVariable("APP_ENV")
	mdbUri = goDotEnvVariable("MONGO_URI")
	if mdbUri == "" {
		log.Fatal("Unable to extract 'MONGO_URI' environment variable")
	}

	rootCmd.Flags().IntVarP(
		&Port,
		"port",
		"p",
		3000, "port to run the server on",
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		oscall := <-c
		log.Printf("system call: %v\n", oscall)
		cancel()
	}()

	if err := startServer(ctx); err != nil {
		log.Fatalf("failed to start server: %v\n", err)
	}

	return nil
}

func startServer(ctx context.Context) (err error) {
	portStr := fmt.Sprintf(":%d", Port)

	log.Println("starting server")

	r := api.NewServer()

	srv := &http.Server{
		Handler: r,
		Addr:    portStr,
	}

	go func() {
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %s\n", err)
		}
	}()

	log.Println("paste-server started")

	// Connect to MongoDB and defer disconnection
	r.ConnectDB(mdbUri)
	defer func() {
		r.DisconnectDB()
	}()

	<-ctx.Done()

	log.Println("paste-server stopped")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err = srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("server shutdown failed: %s\n", err)
	}

	log.Println("server exited properly")

	if err == http.ErrServerClosed {
		return nil
	}

	return err
}

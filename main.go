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
	"github.com/h5law/paste-server/cmd"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
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

func main() {
	// Get command line arguments
	cmd.Execute()

	appEnv = goDotEnvVariable("APP_ENV")
	mdbUri = goDotEnvVariable("MONGO_URI")
	if mdbUri == "" {
		log.Fatal("Unable to extract 'MONGO_URI' environment variable")
	}

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

	if err := startServer(ctx); err != nil {
		log.Fatalf("failed to start server: %v\n", err)
	}
}

func startServer(ctx context.Context) (err error) {
	port := viper.GetInt("port")

	portStr := fmt.Sprintf(":%d", port)

	log.Println("starting server")

	r := api.NewServer()

	srv := &http.Server{
		Handler: r,
		Addr:    portStr,
	}

	// Start server in go routine
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

	// Context has been cancelled - stop everything
	<-ctx.Done()

	log.Println("paste-server stopped")

	// Create context and shutdown server
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

/*
Copyright © 2022 Harry Law <hrryslw@pm.me>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
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

	if err := startServer(ctx); err != nil {
		log.Fatalf("failed to start server: %v\n", err)
	}
}

func startServer(ctx context.Context) error {
	port := viper.GetInt("port")
	portStr := fmt.Sprintf(":%d", port)

	// Load connection URI for mongo from .env
	uri := goDotEnvVariable("MONGO_URI")
	if uri == "" {
		log.Fatal("Unable to extract 'MONGO_URI' environment variable")
	}

	h := api.NewHandler()

	log.Println("starting server")

	srv := &http.Server{
		Addr:         portStr,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      h,
	}

	// Start server in go routine so non-blocking
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v\n", err)
		}
	}()

	log.Println("paste-server started")

	// Connect to MongoDB and defer disconnection
	h.ConnectDB(uri)

	// Context has been cancelled - stop everything
	<-ctx.Done()

	log.Println("stopping server")

	// Create context and shutdown server
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.DisconnectDB()
	err := srv.Shutdown(ctxShutdown)
	if err != nil {
		log.Fatalf("server shutdown failed: %v\n", err)
	}

	log.Println("paste-server stopped")

	if err == http.ErrServerClosed {
		return nil
	}

	return err
}

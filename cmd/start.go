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
package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/h5law/paste-server/api"
	log "github.com/h5law/paste-server/logger"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	port       int
	logFile    string
	jsonFormat bool

	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start a paste-server http server",
		Long: `The start subcommand will start a paste-server instance on
the given port or default to using 3000.

If no logfile is given logs will be directed to os.Stdout - if a logfile is
provided logs will be appended to that file (creating it if it doesn't exist).`,
		Run: func(cmd *cobra.Command, args []string) {
			prepareServer()
		},
	}
)

// Load environment varaibles
func goDotEnvVariable(key string) string {
	if err := godotenv.Load(".env"); err != nil {
		log.Print("fatal", "Error loading .env file")
	}
	return os.Getenv(key)
}

func prepareServer() {
	// Enable graceful shutdown on signal interrupts
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	// Listen for interrupt
	go func() {
		oscall := <-c
		log.Print("warn", "system call: %v", oscall)
		cancel()
	}()

	if err := startServer(ctx); err != nil {
		log.Print("fatal", "failed to start server: %v", err)
	}
}

func startServer(ctx context.Context) error {
	port := viper.GetInt("port")
	portStr := fmt.Sprintf(":%d", port)

	// Load connection URI for mongo from .env
	uri := goDotEnvVariable("MONGO_URI")
	if uri == "" {
		log.Print("fatal", "Unable to extract 'MONGO_URI' environment variable")
	}

	h := api.NewHandler()

	log.Print("info", "starting server")

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
			log.Print("fatal", "listen error: %v", err)
		}
	}()

	log.Print("info", "paste-server started on %v", portStr)

	// Connect to MongoDB and defer disconnection
	h.ConnectDB(uri)

	// Context has been cancelled - stop everything
	<-ctx.Done()

	log.Print("info", "stopping server")

	// Create context and shutdown server
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.DisconnectDB()
	err := srv.Shutdown(ctxShutdown)
	if err != nil {
		log.Print("fatal", "server shutdown failed: %v", err)
	}

	log.Print("info", "paste-server stopped")

	if err == http.ErrServerClosed {
		return nil
	}

	return err
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().IntVarP(
		&port,
		"port",
		"p",
		3000, "port to run the server on",
	)
	startCmd.Flags().StringVarP(
		&logFile,
		"logfile",
		"l",
		"", "path to log file",
	)
	startCmd.Flags().BoolVarP(
		&jsonFormat,
		"json",
		"j",
		false, "use json formatting for logs",
	)

	viper.BindPFlag("port", startCmd.Flags().Lookup("port"))
	viper.BindPFlag("logfile", startCmd.Flags().Lookup("logfile"))
	viper.BindPFlag("json", startCmd.Flags().Lookup("json"))
	viper.SetDefault("port", 3000)
	viper.SetDefault("logfile", "")
	viper.SetDefault("json", false)
}

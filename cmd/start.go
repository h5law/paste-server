/*
Copyright © 2022 Harry Law <hrryslw@pm.me>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software
   without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
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

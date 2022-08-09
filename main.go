package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"

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

var flags struct {
	port    int
	verbose bool
	quiet   bool
}

var flagsName = struct {
	port, portShort       string
	verbose, verboseShort string
	quiet, quietShort     string
}{
	"port", "p",
	"verbose", "v",
	"quiet", "q",
}

func print(level int, s string) {
	if flags.quiet {
		return
	}
	if level == 1 {
		log.Println(s)
	}
	if level == 2 && flags.verbose {
		log.Println(s)
	}
}

func printf(level int, s string, args ...interface{}) {
	if flags.quiet {
		return
	}
	if level == 1 {
		log.Printf(s, args...)
	}
	if level == 2 && flags.verbose {
		log.Printf(s, args...)
	}
}

var rootCmd = &cobra.Command{
	Use:   "paste-server",
	Short: "Start a paste-server instance locally",
	Long:  `Start a paste-server instance to allow for CRUD operations for ephemeral pastes on a given port`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func startHttpServer(wg *sync.WaitGroup) *http.Server {
	portStr := fmt.Sprintf(":%d", flags.port)
	srv := &http.Server{Addr: portStr}

	go func() {
		defer wg.Done()

		printf(1, "Starting server on port: %d", flags.port)
		if appEnv == "development" {
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("ListenAndServe(): %v", err)
			}
		}
		print(1, "Shutting down server")
	}()

	return srv
}

func run() error {
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)
	srv := startHttpServer(httpServerExitDone)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		if err := srv.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	httpServerExitDone.Wait()

	return nil
}

func main() {
	appEnv = goDotEnvVariable("APP_ENV")

	rootCmd.Flags().IntVarP(
		&flags.port,
		flagsName.port,
		flagsName.portShort,
		3000, "port to run the server on",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&flags.verbose,
		flagsName.verbose,
		flagsName.verboseShort,
		true, "log verbose output",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&flags.quiet,
		flagsName.quiet,
		flagsName.quietShort,
		false, "suppress all output",
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

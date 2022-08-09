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
}

var flagsName = struct {
	port, portShort       string
	verbose, verboseShort string
}{
	"port", "p",
	"verbose", "v",
}

var print func(s string)
var printf func(s string, args ...interface{})

func logNoop(s string)                       {}
func logNoopf(s string, args ...interface{}) {}

func logOut(s string) {
	log.Println("[paste-server] " + s)
}
func logOutf(s string, args ...interface{}) {
	log.Printf("[paste-sever] "+s, args...)
}

var rootCmd = &cobra.Command{
	Use:   "paste-server",
	Short: "Start a paste-server instance locally",
	Long:  `Start a paste-server instance to allow for CRUD operations for ephemeral pastes on a given port`,
	RunE: func(cmd *cobra.Command, args []string) error {
		print = logNoop
		printf = logNoopf
		if flags.verbose {
			print = logOut
			printf = logOutf
		}
		return run()
	},
}

func startHttpServer(wg *sync.WaitGroup) *http.Server {
	portStr := fmt.Sprintf(":%d", flags.port)
	srv := &http.Server{Addr: portStr}

	go func() {
		defer wg.Done()

		printf("Starting server on port: %d", flags.port)
		if appEnv == "development" {
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("[paste-server]: %v", err)
			}
		}
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
		print("Shutting down server")
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
		false, "log verbose output",
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

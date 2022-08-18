package logger

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func init() {
	// Use JSON formatting
	log.SetFormatter(&log.JSONFormatter{})
}

// Wrapper function exported for use in the rest of module
func Print(level, msg string, args ...interface{}) {
	verbose := viper.GetBool("verbose")

	message := fmt.Sprintf(msg, args...)

	// If logfile flag has been set use this output instead of os.Stdout
	logfile := viper.GetString("logfile")
	if logfile != "" {
		file, err := os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		log.SetOutput(file)
	}

	switch level {
	case "info":
		// Only show info level logs when verbose flag used
		if verbose {
			log.Info(message)
		}
	case "fatal":
		log.Fatal(message)
	case "warn":
		log.Warn(message)
	case "error":
		log.Error(message)
	default:
		log.Fatalf("Unknown logging level: %s\n", level)
	}
}

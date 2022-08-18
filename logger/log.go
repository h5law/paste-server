package logger

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

func Print(level, msg string, args ...interface{}) {
	verbose := viper.GetBool("verbose")

	message := fmt.Sprintf(msg, args...)

	switch level {
	case "info":
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

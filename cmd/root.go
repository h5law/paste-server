package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	port    int
	verbose bool
	logFile string

	rootCmd = &cobra.Command{
		Use:   "paste-server",
		Short: "Start a paste-server instance locally",
		Long: `Start a paste-server instance to interact with a MongoDB
database to create, read, update and delete temporary pastes`,
		Run: func(cmd *cobra.Command, args []string) {
			return
		},
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().IntVarP(
		&port,
		"port",
		"p",
		3000, "port to run the server on",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&verbose,
		"verbose",
		"v",
		false, "print verbose logs",
	)
	rootCmd.PersistentFlags().StringVarP(
		&logFile,
		"logFile",
		"l",
		"", "path to log file",
	)

	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("logFile", rootCmd.PersistentFlags().Lookup("logFile"))
	viper.SetDefault("port", 3000)
	viper.SetDefault("verbose", "false")
	viper.SetDefault("logFile", "")
}

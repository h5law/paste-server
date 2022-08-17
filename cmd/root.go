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
	"fmt"
	"os"

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
		Version: "ALPHA-0.1.1",
		Run: func(cmd *cobra.Command, args []string) {
			return
		}
	}
)

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(err)
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

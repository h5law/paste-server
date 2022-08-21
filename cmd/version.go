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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// versionCmd represents the version command
var (
	VersionVerbose string
	VersionShort   string

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version of paste-server",
		Long:  `Display the exact version number of the paste-server command.`,
		Run: func(cmd *cobra.Command, args []string) {
			verbose := viper.GetBool("verbose")
			if verbose {
				fmt.Println(VersionVerbose)
				return
			}
			fmt.Println(VersionShort)
			return
		},
	}
)

func init() {
	rootCmd.AddCommand(versionCmd)
	VersionVerbose = "print-server (temporary pastebin http server) v1.0-BETA"
	VersionShort = "v1.0-BETA"
}

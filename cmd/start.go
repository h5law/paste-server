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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/h5law/paste-server/api"
	log "github.com/h5law/paste-server/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	port       int
	logFile    string
	jsonFormat bool
	maxUpload  int
	secure     bool
	domain     string
	email      string

	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start a paste-server http server",
		Long: `The start subcommand will start a paste-server instance on
the given port or default to using 3000.

If no logfile is given logs will be directed to os.Stdout - if a logfile is
provided logs will be appended to that file (creating it if it doesn't exist).

To start a paste-server instance in HTTPS mode using TLS you must provide the
-t or --tls flag and also the -d or --domain flag which is a string storing
the domain name of your server and the -e or --email flag. These are for the
LetsEncrypt certificate and are required for the certificates to be created.
You will need ports :80 and :443 both open as the server will redirect certain
HTTP traffic to HTTPS.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			if secure := viper.GetBool("tls"); secure {
				cmd.MarkFlagRequired("domain")
				cmd.MarkFlagRequired("email")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			prepareServer()
		},
	}
)

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
	startCmd.Flags().IntVarP(
		&maxUpload,
		"max-size",
		"",
		1, "max request body size in MB",
	)
	startCmd.Flags().BoolVarP(
		&secure,
		"tls",
		"t",
		false, "use TLS (https) mode for server",
	)
	startCmd.Flags().StringVarP(
		&domain,
		"domain",
		"d",
		"example.com", "domain to use to TLS configuration",
	)
	startCmd.Flags().StringVarP(
		&email,
		"email",
		"e",
		"admin@example.com", "email to use to TLS configuration",
	)

	viper.BindPFlag("port", startCmd.Flags().Lookup("port"))
	viper.BindPFlag("logfile", startCmd.Flags().Lookup("logfile"))
	viper.BindPFlag("json", startCmd.Flags().Lookup("json"))
	viper.BindPFlag("max-size", startCmd.Flags().Lookup("max-size"))
	viper.BindPFlag("tls", startCmd.Flags().Lookup("tls"))
	viper.BindPFlag("domain", startCmd.Flags().Lookup("domain"))
	viper.BindPFlag("email", startCmd.Flags().Lookup("email"))
	viper.SetDefault("port", 3000)
	viper.SetDefault("logfile", "")
	viper.SetDefault("json", false)
	viper.SetDefault("max-size", 1)
	viper.SetDefault("tls", false)
	viper.SetDefault("domain", "example.com")
	viper.SetDefault("email", "admin@example.com")
}

func prepareServer() {
	// Enable graceful shutdown on signal interrupts
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	// Listen for interrupt
	go func() {
		oscall := <-c
		log.Print("warn", "system call: %v", oscall)
		cancel()
	}()

	secure := viper.GetBool("tls")
	if secure {
		if err := startServerTLS(ctx); err != nil {
			log.Print("fatal", "failed to start server: %v", err)
		}
	} else {
		if err := startServer(ctx); err != nil {
			log.Print("fatal", "failed to start server: %v", err)
		}
	}
}

func startServer(ctx context.Context) error {
	port := viper.GetInt("port")
	portStr := fmt.Sprintf(":%d", port)

	// Load connection URI for mongo from .env
	uri := viper.GetString("uri")
	if uri == "" {
		log.Print("fatal", "`uri` not set in config file")
	}

	h := api.NewHandler()

	log.Print("info", "starting server")

	srv := &http.Server{
		Addr:              portStr,
		ReadHeaderTimeout: time.Second * 15,
		WriteTimeout:      time.Second * 15,
		ReadTimeout:       time.Second * 15,
		IdleTimeout:       time.Second * 60,
		Handler:           h,
	}

	// Start server in go routine so non-blocking
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Print("fatal", "listen error: %v", err)
		}
	}()

	log.Print("info", "paste-server started on %v", portStr)
	maxMiB := int64(viper.GetInt("max-size"))
	maxKiB := maxMiB * 1048576
	log.Print("info", "using max-upload size: %dMB (%dKiB)", maxMiB, maxKiB)

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

func startServerTLS(ctx context.Context) error {
	// Load connection URI for mongo from .env
	uri := viper.GetString("uri")
	if uri == "" {
		log.Print("fatal", "`uri` not set in config file")
	}

	h := api.NewHandler()

	log.Print("info", "starting https server")

	// Create TLS config
	email := viper.GetString("email")
	domain := viper.GetString("domain")

	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = email
	// TODO add check for APP_ENV == "test"
	// then use certmagic.LetsEncryptStagingCA
	certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA

	cfg := certmagic.NewDefault()
	err := cfg.ManageSync(context.TODO(), []string{domain, "www." + domain})
	if err != nil {
		return err
	}

	// Create HTTP and HTTPS listeners
	httpLn, err := net.Listen("tcp", ":80")
	if err != nil {
		return err
	}
	httpsLn, err := tls.Listen("tcp", ":443", cfg.TLSConfig())
	if err != nil {
		httpLn = nil
		return err
	}

	defer func() {
		httpLn.Close()
		httpsLn.Close()
	}()

	// Create servers
	httpSrv := &http.Server{
		ReadHeaderTimeout: time.Second * 15,
		WriteTimeout:      time.Second * 15,
		ReadTimeout:       time.Second * 15,
		IdleTimeout:       time.Second * 60,
	}
	if am, ok := cfg.Issuers[0].(*certmagic.ACMEIssuer); ok {
		httpSrv.Handler = am.HTTPChallengeHandler(http.HandlerFunc(httpRedirectHandler))
	}

	httpsSrv := &http.Server{
		ReadHeaderTimeout: time.Second * 15,
		WriteTimeout:      time.Second * 15,
		ReadTimeout:       time.Second * 15,
		IdleTimeout:       time.Second * 60,
		Handler:           h,
	}

	// Start servers in go routines so non-blocking
	go func() {
		err := httpSrv.Serve(httpLn)
		if err != nil && err != http.ErrServerClosed {
			log.Print("fatal", "(http) listen error: %v", err)
		}
	}()
	log.Print("info", "paste-server started on :80 redirecting to :443")

	go func() {
		err := httpsSrv.Serve(httpsLn)
		if err != nil && err != http.ErrServerClosed {
			log.Print("fatal", "(https) listen error: %v", err)
		}
	}()
	log.Print("info", "paste-server started on :443")

	maxMiB := int64(viper.GetInt("max-size"))
	maxKiB := maxMiB * 1048576
	log.Print("info", "using max-upload size: %dMB (%dKiB)", maxMiB, maxKiB)

	// Connect to MongoDB and defer disconnection
	h.ConnectDB(uri)

	// Context has been cancelled - stop everything
	<-ctx.Done()

	log.Print("info", "stopping server")

	// Create context and shutdown server
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.DisconnectDB()

	// Shutdown both servers
	err = httpSrv.Shutdown(ctxShutdown)
	if err != nil {
		log.Print("fatal", "(http) server shutdown failed: %v", err)
	}

	err = httpsSrv.Shutdown(ctxShutdown)
	if err != nil {
		log.Print("fatal", "(https) server shutdown failed: %v", err)
	}

	log.Print("info", "paste-server stopped")

	if err == http.ErrServerClosed {
		return nil
	}

	return err
}

func httpRedirectHandler(w http.ResponseWriter, r *http.Request) {
	toURL := "https://"

	// redirect to standard :443 so no need for port
	requestHost := hostOnly(r.Host)

	toURL += requestHost
	toURL += r.URL.RequestURI()

	w.Header().Set("Connection", "close")

	http.Redirect(w, r, toURL, http.StatusMovedPermanently)
}

func hostOnly(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport // OK; probably had no port to begin with
	}
	return host
}

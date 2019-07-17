package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
)

func main() {
	var parameters WhSvrParameters

	// Get command line parameters
	flag.IntVar(&parameters.port, "port", 8443, "Webhook server port.")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/validating-admission-webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/validating-admission-webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.StringVar(&parameters.configFile, "configFile", "/validating-admission-webhook/config.yaml", "File containing validation rules.")
	flag.Parse()

	// Load certificates
	pair, err := tls.LoadX509KeyPair(parameters.certFile, parameters.keyFile)
	if err != nil {
		glog.Errorf("Failed to load key pair: %v", err)
	}

	// Create new WebhookServer instance
	whsvr := NewWebhookServer(parameters.port, pair)

	// Read and parse config
	whsvr.readConfig(parameters.configFile)

	// Define http server and server handler
	mux := http.NewServeMux()

	// Register path handlers
	mux.HandleFunc("/validate", whsvr.serve)
	whsvr.server.Handler = mux

	// Start webhook server in new goroutine
	go func() {
		if err := whsvr.server.ListenAndServeTLS("", ""); err != nil {
			glog.Errorf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	glog.Info("Listening for incoming requests...")

	// Listen for OS shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	glog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	if err := whsvr.server.Shutdown(context.Background()); err != nil {
		glog.Errorf("Failed to shut down webhook server gracefully: %v", err)
	}
}

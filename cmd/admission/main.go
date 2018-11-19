package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"

	"github.com/phoracek/network-attachment-definition-pod-admission/pkg/admission"
)

func main() {
	var port int
	var certFile, keyFile, configFile string

	// get command line parameters
	flag.IntVar(&port, "port", 443, "Webhook server port.")
	flag.StringVar(&certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.StringVar(&configFile, "configFile", "/etc/webhook/config/config.pem", "File containing webhook configuration and patching rules.")
	flag.Parse()

	// load certificate and key
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		glog.Errorf("Failed to load key pair: %v", err)
	}

	// load config
	config, err := admission.LoadConfig(configFile)
	if err != nil {
		glog.Errorf("Failed to load config: %v", err)
	}

	// initialize webhook server
	whsvr := &admission.WebhookServer{
		Config: config,
		Server: &http.Server{
			Addr:      fmt.Sprintf(":%v", port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", whsvr.Serve)
	whsvr.Server.Handler = mux

	// start webhook server in new rountine
	go func() {
		if err := whsvr.Server.ListenAndServeTLS("", ""); err != nil {
			glog.Fatalf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	// listening to OS shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
	glog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	whsvr.Server.Shutdown(context.Background())
}

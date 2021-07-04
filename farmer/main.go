package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	log "github.com/taigrr/log-socket/logger"

	"github.com/gogrlx/grlx/api"
	certs "github.com/gogrlx/grlx/certs"
	. "github.com/gogrlx/grlx/config"

	// . "github.com/gogrlx/grlx/types"
	"github.com/nats-io/nats-server/v2/server"
	nats_server "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

func init() {
	log.SetLogLevel(log.LTrace)
	createConfigRoot()
}

func main() {
	defer log.Flush()
	certs.GenCert([]string{"grlx"})
	RunNATSServer(&DefaultTestOptions)
	StartAPIServer()
	go ConnectFarmer()
	select {}

	// Create ca + cert
	// Spin up mux server
	// Serve ca + cert over insecure tls
	// Load cert into keychain
	// Generate nkey and save or read existing
	// Post user struct to mux
	// Attempt nats auth
	// Auth nats bus
	// Cli accept key, add to config file
	// Update auth users via api

}

func createConfigRoot() {
	_, err := os.Stat(ConfigRoot)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		err = os.MkdirAll(ConfigRoot, os.ModePerm)
		if err != nil {
			log.Panicf(err.Error())
		}
	}
}

func StartAPIServer() {
	r := api.NewRouter(BuildInfo, CertFile)
	srv := http.Server{
		Addr:         "0.0.0.0:5405",
		WriteTimeout: time.Second * 120,
		ReadTimeout:  time.Second * 120,
		IdleTimeout:  time.Second * 120,
		Handler:      r,
	}
	go func() {
		if err := srv.ListenAndServeTLS(CertFile, KeyFile); err != nil {
			log.Fatalf(err.Error())
		}
	}()

}

// RunNATSServer starts a new Go routine based server
func RunNATSServer(opts *server.Options) {
	opts = &DefaultTestOptions
	// Optionally override for individual debugging of tests
	err := opts.ProcessConfigFile("config.json")
	if err != nil {
		log.Panicf("Error configuring server: %v", err)
	}

	s, err := nats_server.NewServer(opts)
	if err != nil || s == nil {
		log.Panicf("No NATS Server object returned: %v", err)
	}
	if err != nil || s == nil {
		log.Panicf("No NATS Server object returned: %v", err)
	}
	// Run server in Go routine.
	go s.Start()

	// Wait for accept loop(s) to be started
	if !s.ReadyForConnections(10 * time.Second) {
		log.Panicf("Unable to start NATS Server in Go Routine")
	}
	s.ReloadOptions(opts)
}

func ConnectFarmer() {
	var connectionAttempts = 0
	opt, err := nats.NkeyOptionFromSeed("seed.txt")
	if err != nil {
		//TODO: handle error
		log.Panic(err)
	}
	certPool := x509.NewCertPool()
	rootPEM, err := ioutil.ReadFile(CertFile)
	if err != nil || rootPEM == nil {
		log.Panicf("nats: error loading or parsing rootCA file: %v", err)
	}
	ok := certPool.AppendCertsFromPEM(rootPEM)
	if !ok {
		log.Errorf("nats: failed to parse root certificate from %q", CertFile)
	}
	config := &tls.Config{
		ServerName: "localhost",
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	nc, err := nats.Connect("nats://localhost:4443", nats.Secure(config), opt,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(1),
		nats.ReconnectWait(time.Second),

		nats.ReconnectHandler(func(_ *nats.Conn) {
			connectionAttempts++
			log.Warnf("WARN: Reconnecting Farmer to NATS bus, attempt: %d\n", connectionAttempts)
		}),
	)
	if err != nil {
		log.Errorf("Got an error on Connect with Secure Options: %+v\n", err)
	}
	log.Debugf("Successfully joined Farmer to NATS bus")

	//	nc, err := nats.Connect(serverUrl, opt)
	//	if err != nil {
	//		//TODO: handle error
	//		panic(err)
	//	}
	ec, _ := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	defer ec.Close()
	// pull in nkeys dep for later
	nkeys.FromSeed([]byte(""))
	select {}
}

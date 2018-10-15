package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	ld "gopkg.in/launchdarkly/go-client.v2"
)

type key int

const (
	requestIDKey key = 0
)

var (
	listenAddr string
	healthy    int32
	body       []byte
)

func main() {
	// Default to port 5000 on localhost
	flag.StringVar(&listenAddr, "listen-addr", ":3000", "server listen address")
	flag.Parse()

	logger := log.New(os.Stdout, "http: ", log.LstdFlags)

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging(logger)(routes())),
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// Listen for CTRL+C or kill and start shutting down the app without
	// disconnecting people by not taking any new requests. ("Graceful Shutdown")
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Println("Server is shutting down...")
		atomic.StoreInt32(&healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()

	logger.Println("Server is ready to handle requests at", listenAddr)
	atomic.StoreInt32(&healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	<-done
	logger.Println("Server stopped")

}

func routes() *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/", indexHandler)
	router.HandleFunc("/health", healthHandler)
	router.HandleFunc("/ping", pingHandler)
	return router
}

func getFeatureFlag(name string) bool {

	client, _ := ld.MakeClient("sdk-00f0cbf6-abc8-48a4-b866-a5a1f7a53cd8", 5*time.Second)

	key := "bob@example.com"
	first := "Bob"
	last := "Loblaw"
	custom := map[string]interface{}{"groups": "beta_testers"}

	user := ld.User{
		Key:       &key,
		FirstName: &first,
		LastName:  &last,
		Custom:    &custom,
	}

	feature, _ := client.BoolVariation(name, user, false)

	client.Close()
	return feature
}

func indexHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	var (
		color string
		fFlag = "False"
	)

	if getFeatureFlag("blue-header") {
		// application code to show the feature
		color = "blue"
		fFlag = "True"
	} else {
		// the code to run if the feature is off
		color = "red"
	}

	type data struct {
		Color       string
		FeatureFlag string
	}

	sdata := data{
		Color:       color,
		FeatureFlag: fFlag,
	}

	tmpl, err := template.New("index").Parse(indexHtml)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	spew.Dump(sdata)
	if err := tmpl.Execute(w, sdata); err != nil {
		fmt.Println(err)
	}
}

// pingHandler -
// Simple health check.
func pingHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "pong!")
}

// forceTextHandler -
// Prevent Content-Type sniffing
func forceTextHandler(w http.ResponseWriter, r *http.Request) {
	// https://stackoverflow.com/questions/18337630/what-is-x-content-type-options-nosniff
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "{\"status\":\"ok\"}")
}

// healthHandler -
// Report server status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintln(w, "{\"status\":\"ok\"}")
}

// logging just a simple logging handler
// this generates a basic access log entry
func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// tracing for debuging a access log entry to a given request
func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

var indexHtml = `<!doctype html>
<html>
<head>
<!-- Compiled and minified CSS -->
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/materialize/0.97.0/css/materialize.min.css">

<!-- Compiled and minified JavaScript -->
<script src="https://cdnjs.cloudflare.com/ajax/libs/materialize/0.97.0/js/materialize.min.js"></script>
<title>Frontend Web Server</title>
</head>
<body>
<div class="container">
<div class="row">
<div class="col s2">&nbsp;</div>
<div class="col s8">


<div class="card {{.Color}}">
<div class="card-content white-text">
<div class="card-title">Feature One Demo</div>
</div>
<div class="card-content white">
    Color is currently {{.Color}}, feature flag is currently {{.FeatureFlag}}
    <br/>
</div>
</div>
</div>
</div>
<div class="col s2">&nbsp;</div>
</div>
</div>
</html>`

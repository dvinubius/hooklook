package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	publicBaseURLEnvironmentVariable       = "PUBLIC_BASE_URL"
	maxRequestBodyBytesEnvironmentVariable = "MAX_REQUEST_BODY_BYTES"
	defaultMaxRequestBodyBytes             = 256 << 10
	defaultMaxRequestHeaderBytes           = 32 << 10
	defaultReadHeaderTimeout               = 5 * time.Second
	defaultReadTimeout                     = 15 * time.Second
	defaultIdleTimeout                     = 60 * time.Second
	databasePath                           = "hooklook.db"
)

var (
	publicBaseURL       string
	maxRequestBodyBytes int64 = defaultMaxRequestBodyBytes
)

var store *Store

func publicBaseURLFromEnvironment() (string, error) {
	value := strings.TrimRight(os.Getenv(publicBaseURLEnvironmentVariable), "/")
	if value == "" {
		return "", fmt.Errorf("%s is required", publicBaseURLEnvironmentVariable)
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%s must be an absolute URL", publicBaseURLEnvironmentVariable)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s must use http or https", publicBaseURLEnvironmentVariable)
	}

	return value, nil
}

func maxRequestBodyBytesFromEnvironment() (int64, error) {
	value := os.Getenv(maxRequestBodyBytesEnvironmentVariable)
	if value == "" {
		return defaultMaxRequestBodyBytes, nil
	}

	limit, err := strconv.ParseInt(value, 10, 64)
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", maxRequestBodyBytesEnvironmentVariable)
	}

	return limit, nil
}

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)

	mux.HandleFunc("POST /api/bins", createBin)
	mux.HandleFunc("GET /api/bins/{code}/requests", getBinRequests)

	mux.HandleFunc("GET /admin/bins", getAllBins)

	mux.HandleFunc("/b/{code}", captureRequest)
	mux.HandleFunc("/b/{code}/{path...}", captureRequest)

	return mux
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		MaxHeaderBytes:    defaultMaxRequestHeaderBytes,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
}

func config() {
	configuredPublicBaseURL, err := publicBaseURLFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	publicBaseURL = configuredPublicBaseURL
	configuredMaxRequestBodyBytes, err := maxRequestBodyBytesFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	maxRequestBodyBytes = configuredMaxRequestBodyBytes
}

func main() {
	config()

	db, err := openDB(databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatal(err)
	}

	store = newBinStore(db)

	log.Fatal(newHTTPServer(":8080", routes()).ListenAndServe())
}

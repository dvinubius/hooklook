package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	publicBaseURLEnvironmentVariable = "PUBLIC_BASE_URL"
)

var (
	publicBaseURL string
	store         = newBinStore()
)

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

func main() {
	configuredPublicBaseURL, err := publicBaseURLFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	publicBaseURL = configuredPublicBaseURL

	log.Fatal(http.ListenAndServe(":8080", routes()))
}

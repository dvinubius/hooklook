package main

import (
	"encoding/json"
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

func health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK, I'm healthy")
}

func createBin(w http.ResponseWriter, req *http.Request) {
	bin, err := store.create()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}{Code: bin.Code, URL: publicBaseURL + "/b/" + bin.Code}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func main() {
	configuredPublicBaseURL, err := publicBaseURLFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	publicBaseURL = configuredPublicBaseURL

	http.HandleFunc("GET /health", health)
	http.HandleFunc("POST /api/bins", createBin)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

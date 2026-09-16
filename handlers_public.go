package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK, I'm healthy")
}

func createBin(w http.ResponseWriter, req *http.Request) {
	bin, err := store.createBin()
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

func captureRequest(w http.ResponseWriter, req *http.Request) {
	binCode := req.PathValue("code")

	path := strings.TrimPrefix(req.URL.Path, "/b/"+binCode)
	parsedRequest, err := parseRequest(req, path, maxRequestBodyBytes)
	if errors.Is(err, ErrRequestBodyTooLarge) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	requestCode, err := store.saveRequest(parsedRequest, binCode)
	if errors.Is(err, ErrBinNotFound) {
		http.Error(w, "bin not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, ErrBinFull) {
		http.Error(w, "bin storage limit reached", http.StatusInsufficientStorage)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := struct {
		Id  string `json:"id"`
		URL string `json:"url"`
	}{Id: requestCode, URL: publicBaseURL + "/bins/" + binCode + "/requests/" + requestCode}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func getBinRequests(w http.ResponseWriter, req *http.Request) {
	binCode := req.PathValue("code")

	requests, err := store.getBinRequests(binCode)
	if errors.Is(err, ErrBinNotFound) {
		http.Error(w, "bin not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(requests)
}

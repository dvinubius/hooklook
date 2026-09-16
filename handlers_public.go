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
	eventHub.openBin(bin.Code)

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

	requestId, err := store.saveRequest(parsedRequest, binCode)
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

	eventHub.publish(binCode, SummarizedRequest{
		Id:          requestId,
		Method:      parsedRequest.Method,
		Path:        parsedRequest.Path,
		RawQuery:    parsedRequest.RawQuery,
		ReceivedAt:  parsedRequest.ReceiptTime,
		ContentType: parsedRequest.ContentType,
		BodySizeKiB: parsedRequest.BodySizeKiB,
		HeaderCount: len(parsedRequest.Headers),
	})

	response := struct {
		Id  string `json:"id"`
		URL string `json:"url"`
	}{Id: requestId, URL: publicBaseURL + "/bins/" + binCode + "/requests/" + requestId}

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

func getBinEvents(w http.ResponseWriter, req *http.Request) {
	binCode := req.PathValue("code")
	if _, err := store.getBinRequests(binCode); err != nil {
		if errors.Is(err, ErrBinNotFound) {
			http.Error(w, "bin not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	events, ok := eventHub.subscribe(binCode)
	if !ok {
		http.Error(w, "server is shutting down", http.StatusServiceUnavailable)
		return
	}
	defer eventHub.unsubscribe(binCode, events)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	responseController := http.NewResponseController(w)
	if err := responseController.Flush(); err != nil {
		return
	}

	for {
		select {
		case <-req.Context().Done():
			return
		case summarizedRequest, ok := <-events:
			if !ok {
				return // channel drained (closed)
			}

			encoded, err := json.Marshal(summarizedRequest)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "event: request\ndata: %s\n\n", encoded); err != nil {
				return
			}
			if err := responseController.Flush(); err != nil {
				return
			}
		}
	}
}

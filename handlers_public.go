package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK, I'm healthy")
}

func captureRequest(w http.ResponseWriter, req *http.Request) {
	binCode := req.PathValue("code")

	path := strings.TrimPrefix(req.URL.Path, "/b/"+binCode)
	parsedRequest, err := parseRequest(req, path)
	if err != nil {
		// An interrupted or malformed client body is a request failure. Caddy
		// may already be returning 413 for a body beyond its edge limit.
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	started := time.Now()
	requestId, err := store.saveRequest(parsedRequest, binCode)
	observeDBOperation("capture", started, err)
	result := "accepted"
	if errors.Is(err, ErrBinNotFound) {
		result = "missing_bin"
	} else if errors.Is(err, ErrBinFull) {
		result = "bin_full"
	} else if errors.Is(err, ErrStoreFull) {
		result = "store_full"
	} else if err != nil {
		result = "internal_error"
	}
	telemetry.captures.WithLabelValues(result).Inc()
	if result == "bin_full" || result == "store_full" {
		slog.Warn("capture_capacity_rejected", "reason", result)
	}
	if err == nil {
		telemetry.operations.WithLabelValues("capture").Inc()
	}
	if errors.Is(err, ErrBinNotFound) {
		http.Error(w, "bin not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, ErrBinFull) {
		w.Header().Set("X-Hooklook-Error", "bin_full")
		http.Error(w, "bin storage limit reached", http.StatusInsufficientStorage)
		return
	}
	if errors.Is(err, ErrStoreFull) {
		w.Header().Set("X-Hooklook-Error", "store_full")
		http.Error(w, "global storage limit reached", http.StatusInsufficientStorage)
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
	if _, ok := authorizedAccess(w, req); !ok {
		return
	}

	started := time.Now()
	requests, err := store.getBinRequests(binCode)
	observeDBOperation("list", started, err)
	if err == nil {
		telemetry.operations.WithLabelValues("list").Inc()
	}
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
	streamAccessMu.Lock()
	access, ok := authorizedAccess(w, req)
	if !ok {
		streamAccessMu.Unlock()
		return
	}
	events, ok := eventHub.subscribe(binCode, access.Owner)
	streamAccessMu.Unlock()
	if !ok {
		http.Error(w, "server is shutting down", http.StatusServiceUnavailable)
		return
	}
	reason := "closed"
	telemetry.sseConnections.Inc()
	telemetry.sseEvents.WithLabelValues("open").Inc()
	slog.Info("sse_opened")
	defer func() {
		eventHub.unsubscribe(binCode, events)
		telemetry.sseConnections.Dec()
		telemetry.sseEvents.WithLabelValues(reason).Inc()
		slog.Info("sse_closed", "reason", reason)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Open with a comment line — EventSource ignores it — so the response has a
	// body from the start. A proxy that holds headers back until the first body
	// bytes (the Vite dev server does) would otherwise keep the client's `open`
	// waiting for the first capture.
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		reason = "write_failure"
		return
	}
	responseController := http.NewResponseController(w)
	if err := responseController.Flush(); err != nil {
		reason = "flush_failure"
		return
	}

	for {
		select {
		case <-req.Context().Done():
			reason = "client_disconnect"
			return
		case summarizedRequest, ok := <-events:
			if !ok {
				reason = "channel_closed"
				return // channel drained (closed)
			}

			if summarizedRequest.Id == "" {
				if _, err := fmt.Fprint(w, "event: refresh\ndata: {}\n\n"); err != nil {
					reason = "write_failure"
					return
				}
				if err := responseController.Flush(); err != nil {
					reason = "flush_failure"
					return
				}
				continue
			}
			encoded, err := json.Marshal(summarizedRequest)
			if err != nil {
				reason = "encode_failure"
				telemetry.sseEvents.WithLabelValues("invariant_failure").Inc()
				return
			}
			if _, err := fmt.Fprintf(w, "event: request\ndata: %s\n\n", encoded); err != nil {
				reason = "write_failure"
				return
			}
			if err := responseController.Flush(); err != nil {
				reason = "flush_failure"
				return
			}
		}
	}
}

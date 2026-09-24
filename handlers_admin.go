package main

import (
	"encoding/json"
	"net/http"
	"time"
)

type GlobalStorageStats struct {
	StoreCapacity
	UsedBytes   int64   `json:"usedBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

func getAllBins(w http.ResponseWriter, req *http.Request) {
	started := time.Now()
	bins, err := store.getAllBins()
	observeDBOperation("admin_list", started, err)
	if err == nil {
		telemetry.operations.WithLabelValues("admin_list").Inc()
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bins)
}

func getStorageStats(w http.ResponseWriter, req *http.Request) {
	capacity, err := store.storeCapacity()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	usedBytes := max(int64(0), capacity.DatabaseBytes-capacity.ReusableBytes)
	stats := GlobalStorageStats{
		StoreCapacity: capacity,
		UsedBytes:     usedBytes,
		UsedPercent:   float64(usedBytes) / float64(capacity.MaxBytes) * 100,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

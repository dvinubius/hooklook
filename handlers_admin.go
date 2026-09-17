package main

import (
	"encoding/json"
	"net/http"
)

func getAllBins(w http.ResponseWriter, req *http.Request) {
	bins, err := store.getAllBins()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bins)
}

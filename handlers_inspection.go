package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const ownerCookieName = "hooklook_owner"

var resolveMu sync.Mutex
var streamAccessMu sync.Mutex

func ownerSecret(req *http.Request) string {
	cookie, err := req.Cookie(ownerCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setOwnerCookie(w http.ResponseWriter, secret string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: ownerCookieName, Value: secret, Path: "/", Expires: expires, HttpOnly: true, Secure: strings.HasPrefix(publicBaseURL, "https://"), SameSite: http.SameSiteLaxMode})
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func resolveOwnBin(w http.ResponseWriter, req *http.Request) (Bin, bool) {
	resolveMu.Lock()
	defer resolveMu.Unlock()
	bin, err := store.ownedBin(ownerSecret(req))
	if err == nil {
		setOwnerCookie(w, ownerSecret(req), bin.ExpiresAt)
		return bin, true
	}
	if !errors.Is(err, ErrBinNotFound) {
		http.Error(w, "internal server error", 500)
		return Bin{}, false
	}
	streamAccessMu.Lock()
	started := time.Now()
	bin, secret, err := store.createOwnedBin()
	observeDBOperation("create", started, err)
	if err != nil {
		streamAccessMu.Unlock()
		if errors.Is(err, ErrStoreFull) {
			slog.Warn("bin_creation_capacity_rejected", "reason", "store_full")
			writeCapacityShell(w)
		} else {
			http.Error(w, "internal server error", 500)
		}
		return Bin{}, false
	}
	eventHub.openBin(bin.Code)
	telemetry.operations.WithLabelValues("create").Inc()
	streamAccessMu.Unlock()
	setOwnerCookie(w, secret, bin.ExpiresAt)
	return bin, true
}

func home(w http.ResponseWriter, req *http.Request) {
	noStore(w)
	bin, ok := resolveOwnBin(w, req)
	if !ok {
		return
	}
	http.Redirect(w, req, "/bins/"+bin.Code, http.StatusSeeOther)
}

// withoutTrailingSlash sends a page URL written with a trailing slash to the
// one the application answers on. The query string is kept — a guest's
// invitation lives there — and nothing is looked up first: the canonical URL
// is then authorized like any other visit, so this reveals nothing.
func withoutTrailingSlash(w http.ResponseWriter, req *http.Request) {
	target := strings.TrimRight(req.URL.EscapedPath(), "/")
	if req.URL.RawQuery != "" {
		target += "?" + req.URL.RawQuery
	}
	http.Redirect(w, req, target, http.StatusMovedPermanently)
}

// toBinPage sends `/bins/{code}/requests`, with or without a trailing slash —
// a detail URL with its id cut off — to the bin page itself. Like
// withoutTrailingSlash it keeps the query and looks nothing up first.
func toBinPage(w http.ResponseWriter, req *http.Request) {
	target := "/bins/" + url.PathEscape(req.PathValue("code"))
	if req.URL.RawQuery != "" {
		target += "?" + req.URL.RawQuery
	}
	http.Redirect(w, req, target, http.StatusMovedPermanently)
}

// inspectorPage serves the application for both `/bins/{code}` and the detail
// URL `/bins/{code}/requests/{id}` that a capture reports. Authorization and
// unavailable-target classification happens here, before any document is
// written; the request id is resolved inside the application afterwards. A
// guest's invitation stays in the query string and is read there too.
func inspectorPage(w http.ResponseWriter, req *http.Request) {
	noStore(w)
	code := req.PathValue("code")
	access, err := store.access(code, ownerSecret(req), req.URL.Query().Get("invite"))
	if err != nil {
		if !errors.Is(err, ErrBinNotFound) {
			http.Error(w, "internal server error", 500)
			return
		}
		state := unavailableBinState(req)
		if state == "bin_expired" {
			writeBinExpiredShell(w)
			return
		}
		writeSharedBinUnavailableShell(w)
		return
	}
	if access.Owner {
		setOwnerCookie(w, ownerSecret(req), access.Bin.ExpiresAt)
	}
	writePageShell(w)
}

func unavailableBinState(req *http.Request) string {
	if req.URL.Query().Get("invite") != "" {
		return "shared_bin_unavailable"
	}
	return "bin_expired"
}

func authorizedAccess(w http.ResponseWriter, req *http.Request) (BinAccess, bool) {
	noStore(w)
	access, err := store.access(req.PathValue("code"), ownerSecret(req), req.URL.Query().Get("invite"))
	if errors.Is(err, ErrBinNotFound) {
		state := unavailableBinState(req)
		w.Header().Set("X-Hooklook-Error", state)
		http.Error(w, "bin not found", 404)
		return BinAccess{}, false
	}
	if err != nil {
		http.Error(w, "internal server error", 500)
		return BinAccess{}, false
	}
	if access.Owner {
		setOwnerCookie(w, ownerSecret(req), access.Bin.ExpiresAt)
	}
	return access, true
}

func binInfo(w http.ResponseWriter, req *http.Request) {
	access, ok := authorizedAccess(w, req)
	if !ok {
		return
	}
	capacity, err := store.binCapacity(access.Bin.Code)
	if errors.Is(err, ErrBinNotFound) {
		http.Error(w, "bin not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	access.Capacity = capacity
	access.Bin.TotalBodyBytes = capacity.BodyBytesUsed
	storeCapacity, err := store.storeCapacity()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	access.StoreCapacity = storeCapacity
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(access)
}

func requestDetail(w http.ResponseWriter, req *http.Request) {
	if _, ok := authorizedAccess(w, req); !ok {
		return
	}
	started := time.Now()
	detail, err := store.requestDetail(req.PathValue("code"), req.PathValue("id"))
	observeDBOperation("detail", started, err)
	if err == nil {
		telemetry.operations.WithLabelValues("detail").Inc()
	}
	if errors.Is(err, ErrRequestNotFound) {
		http.Error(w, "request not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

func requireOwnerMutation(w http.ResponseWriter, req *http.Request) bool {
	access, ok := authorizedAccess(w, req)
	if !ok {
		return false
	}
	if !access.Owner {
		http.Error(w, "forbidden", 403)
		return false
	}
	origin := req.Header.Get("Origin")
	base, err := url.Parse(publicBaseURL)
	if err != nil || origin != base.Scheme+"://"+base.Host || req.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "same-origin request required", 403)
		return false
	}
	return true
}

func deleteOneRequest(w http.ResponseWriter, req *http.Request) {
	if !requireOwnerMutation(w, req) {
		return
	}
	started := time.Now()
	err := store.deleteRequest(req.PathValue("code"), req.PathValue("id"))
	observeDBOperation("delete", started, err)
	if errors.Is(err, ErrRequestNotFound) {
		http.Error(w, "request not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	telemetry.operations.WithLabelValues("delete").Inc()
	eventHub.publish(req.PathValue("code"), SummarizedRequest{})
	w.WriteHeader(http.StatusNoContent)
}

func clearBinRequests(w http.ResponseWriter, req *http.Request) {
	if !requireOwnerMutation(w, req) {
		return
	}
	started := time.Now()
	err := store.clearRequests(req.PathValue("code"))
	observeDBOperation("clear", started, err)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	telemetry.operations.WithLabelValues("clear").Inc()
	eventHub.publish(req.PathValue("code"), SummarizedRequest{})
	w.WriteHeader(http.StatusNoContent)
}

func sharingSetting(w http.ResponseWriter, req *http.Request) {
	if !requireOwnerMutation(w, req) {
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	streamAccessMu.Lock()
	defer streamAccessMu.Unlock()
	if err := store.setSharing(req.PathValue("code"), body.Enabled); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if !body.Enabled {
		// Only the guests lose their stream. The owner asked for this from
		// their own live page, and that page stays live.
		eventHub.closeGuestStreams(req.PathValue("code"))
	}
	w.WriteHeader(http.StatusNoContent)
}

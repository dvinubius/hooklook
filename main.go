package main

import (
	"context"
	"crypto/subtle"
	"errors"
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
	adminTokenEnvironmentVariable          = "ADMIN_TOKEN"
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
	adminToken          string
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

func adminTokenFromEnvironment() (string, error) {
	value := os.Getenv(adminTokenEnvironmentVariable)
	if value == "" {
		return "", fmt.Errorf("%s is required", adminTokenEnvironmentVariable)
	}
	return value, nil
}

// ------------ AUTH MIDDLEWARE

func bearerToken(req *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(req.Header.Get("Authorization"), " ")
	return token, ok && strings.EqualFold(scheme, "Bearer") && token != ""
}

func requireBearerToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		providedToken, ok := bearerToken(req)
		validToken := ok &&
			subtle.ConstantTimeCompare([]byte(providedToken), []byte(token)) == 1
		if !validToken {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, req)
	})
}

// creationTokenUsesLeftKey carries the token uses remaining after this request
// from the authorization middleware, which spends the use, to the handler,
// which reports the remainder to the caller.
type creationTokenUsesLeftKey struct{}

// creationTokenUsesLeft reports how many uses the request's creation token has
// left. The second result is false when the request did not pass through
// requireCreationToken.
func creationTokenUsesLeft(ctx context.Context) (int, bool) {
	usesLeft, ok := ctx.Value(creationTokenUsesLeftKey{}).(int)
	return usesLeft, ok
}

func requireCreationToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token, ok := bearerToken(req)
		if !ok {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		usesLeft, err := store.consumeCreationToken(token)
		if err != nil {
			if errors.Is(err, ErrCreationTokenInvalid) {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(req.Context(), creationTokenUsesLeftKey{}, usesLeft)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)

	mux.Handle("POST /api/bins", requireCreationToken(http.HandlerFunc(createBin)))
	mux.HandleFunc("GET /api/bins/{code}/requests", getBinRequests)

	mux.Handle("GET /admin/bins", requireBearerToken(adminToken, http.HandlerFunc(getAllBins)))
	mux.Handle("POST /admin/tokens", requireBearerToken(adminToken, http.HandlerFunc(issueCreationToken)))
	mux.Handle("GET /admin/tokens", requireBearerToken(adminToken, http.HandlerFunc(getCreationTokens)))
	mux.Handle("DELETE /admin/tokens/{id}", requireBearerToken(adminToken, http.HandlerFunc(revokeCreationToken)))
	mux.Handle("DELETE /admin/bins/{code}", requireBearerToken(adminToken, http.HandlerFunc(deleteBinByCode)))

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
	configuredAdminToken, err := adminTokenFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	adminToken = configuredAdminToken
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

package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	adminTokenEnvironmentVariable    = "ADMIN_TOKEN"
	publicBaseURLEnvironmentVariable = "PUBLIC_BASE_URL"
	defaultReadHeaderTimeout         = 5 * time.Second
	defaultReadTimeout               = 15 * time.Second
	defaultIdleTimeout               = 60 * time.Second
	shutdownTimeout                  = 10 * time.Second
	databasePath                     = "hooklook.db"
)

var publicBaseURL string
var adminToken string

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

func adminTokenFromEnvironment() (string, error) {
	token := os.Getenv(adminTokenEnvironmentVariable)
	if token == "" {
		return "", fmt.Errorf("%s is required", adminTokenEnvironmentVariable)
	}
	return token, nil
}

func requireAdminToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		scheme, provided, ok := strings.Cut(req.Header.Get("Authorization"), " ")
		valid := ok && strings.EqualFold(scheme, "Bearer") && provided != "" &&
			subtle.ConstantTimeCompare([]byte(provided), []byte(adminToken)) == 1
		if !valid {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /{$}", home)
	mux.HandleFunc("GET /bins/{code}", inspectorPage)
	mux.HandleFunc("GET /bins/{code}/requests/{id}", inspectorPage)
	mux.HandleFunc("GET /bins/{code}/{$}", withoutTrailingSlash)
	mux.HandleFunc("GET /bins/{code}/requests/{id}/{$}", withoutTrailingSlash)
	mux.HandleFunc("GET /bins/{code}/requests", toBinPage)
	mux.HandleFunc("GET /bins/{code}/requests/{$}", toBinPage)
	mux.HandleFunc("GET /assets/{path...}", builtAsset(hashedAssetCache))
	mux.HandleFunc("GET /fonts/{path...}", builtAsset(staticFileCache))
	mux.HandleFunc("GET /api/bins/{code}", binInfo)
	mux.HandleFunc("GET /api/bins/{code}/requests/{id}", requestDetail)
	mux.HandleFunc("DELETE /api/bins/{code}/requests/{id}", deleteOneRequest)
	mux.HandleFunc("DELETE /api/bins/{code}/requests", clearBinRequests)
	mux.HandleFunc("PUT /api/bins/{code}/sharing", sharingSetting)

	mux.HandleFunc("GET /api/bins/{code}/requests", getBinRequests)
	mux.HandleFunc("GET /api/bins/{code}/events", getBinEvents)
	mux.Handle("GET /admin/bins", requireAdminToken(http.HandlerFunc(getAllBins)))

	mux.HandleFunc("/b/{code}", captureRequest)
	mux.HandleFunc("/b/{code}/{path...}", captureRequest)

	return mux
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
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
	configuredAdminToken, err := adminTokenFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	adminToken = configuredAdminToken
}

func run(ctx context.Context, address string, logger *slog.Logger) error {
	config()

	db, err := openDB(databasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return err
	}

	store = newBinStore(db)
	server := newHTTPServer(address, routes())
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("shutting down HTTP server")
		eventHub.close()
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		return nil
	}
}

// createDevelopmentBin is an operator-only CLI fixture until the home page
// creates cookie-associated bins. It does not expose another HTTP route.
func createDevelopmentBin(path string) (string, error) {
	configuredPublicBaseURL, err := publicBaseURLFromEnvironment()
	if err != nil {
		return "", err
	}
	db, err := openDB(path)
	if err != nil {
		return "", err
	}
	defer db.Close()
	if err := migrate(db); err != nil {
		return "", err
	}
	bin, err := newBinStore(db).createBin()
	if err != nil {
		return "", err
	}
	return configuredPublicBaseURL + "/b/" + bin.Code, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) == 2 && os.Args[1] == "dev-bin" {
		url, err := createDevelopmentBin(databasePath)
		if err != nil {
			logger.Error("create development bin", "error", err)
			os.Exit(1)
		}
		fmt.Println(url)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, "127.0.0.1:8080", logger); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

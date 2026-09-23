package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	adminTokenEnvironmentVariable    = "ADMIN_TOKEN"
	listenAddressEnvironmentVariable = "LISTEN_ADDRESS"
	publicBaseURLEnvironmentVariable = "PUBLIC_BASE_URL"
	defaultListenAddress             = "127.0.0.1:8080"
	defaultReadHeaderTimeout         = 5 * time.Second
	defaultReadTimeout               = 15 * time.Second
	defaultIdleTimeout               = 60 * time.Second
	shutdownTimeout                  = 10 * time.Second
	databaseCheckInterval            = time.Second
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

func listenAddressFromEnvironment() (string, error) {
	address := os.Getenv(listenAddressEnvironmentVariable)
	if address == "" {
		return defaultListenAddress, nil
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return "", fmt.Errorf("%s must be a host and port", listenAddressEnvironmentVariable)
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return "", fmt.Errorf("%s must have a port between 1 and 65535", listenAddressEnvironmentVariable)
	}

	return address, nil
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
	mux.HandleFunc("GET /favicon.svg", builtAsset(staticFileCache))
	mux.HandleFunc("GET /api/bins/{code}", binInfo)
	mux.HandleFunc("GET /api/bins/{code}/requests/{id}", requestDetail)
	mux.HandleFunc("DELETE /api/bins/{code}/requests/{id}", deleteOneRequest)
	mux.HandleFunc("DELETE /api/bins/{code}/requests", clearBinRequests)
	mux.HandleFunc("PUT /api/bins/{code}/sharing", sharingSetting)

	mux.HandleFunc("GET /api/bins/{code}/requests", getBinRequests)
	mux.HandleFunc("GET /api/bins/{code}/events", getBinEvents)
	mux.Handle("GET /admin/bins", requireAdminToken(http.HandlerFunc(getAllBins)))
	mux.Handle("GET /admin/storage", requireAdminToken(http.HandlerFunc(getStorageStats)))

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

	store, err = configureStore(db)
	if err != nil {
		return fmt.Errorf("configure storage: %w", err)
	}
	serviceCtx, stopServices := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	databaseMonitorDone := make(chan struct{})
	databaseErrors := make(chan error, 1)
	reportDatabaseError := func(err error) {
		select {
		case databaseErrors <- err:
		default:
		}
	}
	go func() {
		defer close(workerDone)
		cleanupWorker(serviceCtx, store, eventHub, reportDatabaseError)
	}()
	go func() {
		defer close(databaseMonitorDone)
		monitorDatabase(serviceCtx, db, databaseCheckInterval, reportDatabaseError)
	}()
	defer func() {
		stopServices()
		<-workerDone
		<-databaseMonitorDone
	}()
	server := newHTTPServer(address, routes())
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	shutdownServer := func() error {
		stopServices()
		eventHub.close()
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			_ = server.Close()
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		return nil
	}

	select {
	case err := <-serverErrors:
		eventHub.close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-databaseErrors:
		if shutdownErr := shutdownServer(); shutdownErr != nil {
			return fmt.Errorf("database became unavailable (%v); %w", err, shutdownErr)
		}
		return fmt.Errorf("database became unavailable: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down HTTP server")
		return shutdownServer()
	}
}

// createDevelopmentBin writes a bin straight to SQLite and returns its capture
// URL, for work that needs one without a browser — the capture smoke test, or
// a `curl` against a fresh bin. The home page is how bins are normally made,
// and that one is cookie-associated; this bin has no owner cookie, so nothing
// can inspect it through the UI. It stays a CLI subcommand deliberately: an
// HTTP route would be an unauthenticated way to create bins.
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
	developmentStore, err := configureStore(db)
	if err != nil {
		return "", err
	}
	bin, err := developmentStore.createBin()
	if err != nil {
		return "", err
	}
	return configuredPublicBaseURL + "/b/" + bin.Code, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) == 3 && os.Args[1] == "backup" {
		if err := backupDatabase(databasePath, os.Args[2]); err != nil {
			logger.Error("backup failed", "error", err)
			os.Exit(1)
		}
		fmt.Println(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "integrity-check" {
		if err := integrityCheck(os.Args[2]); err != nil {
			logger.Error("integrity check failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("ok")
		return
	}
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

	address, err := listenAddressFromEnvironment()
	if err != nil {
		logger.Error("read listen address", "error", err)
		os.Exit(1)
	}
	if err := run(ctx, address, logger); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

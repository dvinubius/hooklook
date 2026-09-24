package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var telemetry = newTelemetry()

type telemetryState struct {
	registry            *prometheus.Registry
	httpRequests        *prometheus.CounterVec
	httpDuration        *prometheus.HistogramVec
	inFlight            prometheus.Gauge
	operations          *prometheus.CounterVec
	captures            *prometheus.CounterVec
	dbDuration          *prometheus.HistogramVec
	dbOperations        *prometheus.CounterVec
	dbErrors            *prometheus.CounterVec
	cleanupDuration     *prometheus.HistogramVec
	cleanupRuns         *prometheus.CounterVec
	expired             prometheus.Counter
	sseConnections      prometheus.Gauge
	sseEvents           *prometheus.CounterVec
	storage             *prometheus.GaugeVec
	storageRatio        prometheus.Gauge
	storageAvailable    prometheus.Gauge
	activeBins          prometheus.Gauge
	activeBinsAvailable prometheus.Gauge
	nearLimit           prometheus.Gauge
	pool                *prometheus.GaugeVec
	poolWait            prometheus.Counter
}

func newTelemetry() *telemetryState {
	t := &telemetryState{registry: prometheus.NewRegistry()}
	t.httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hooklook_http_requests_total", Help: "Completed public HTTP requests."}, []string{"route", "method", "status"})
	t.httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "hooklook_http_request_duration_seconds", Help: "HTTP request duration, excluding SSE.", Buckets: prometheus.DefBuckets}, []string{"route", "method"})
	t.inFlight = prometheus.NewGauge(prometheus.GaugeOpts{Name: "hooklook_http_in_flight_requests", Help: "In-flight HTTP requests excluding SSE."})
	t.operations = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hooklook_bin_operations_total", Help: "Successful bin operations."}, []string{"operation"})
	t.captures = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hooklook_capture_results_total", Help: "Capture results."}, []string{"result"})
	t.dbOperations = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hooklook_db_operations_total", Help: "Selected SQLite operation results."}, []string{"operation", "result"})
	t.dbDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "hooklook_db_operation_duration_seconds", Help: "Selected SQLite operation duration.", Buckets: prometheus.DefBuckets}, []string{"operation"})
	t.dbErrors = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hooklook_db_errors_total", Help: "Selected SQLite operation errors."}, []string{"operation", "kind"})
	t.cleanupDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "hooklook_expiry_cleanup_duration_seconds", Help: "Expiry cleanup run duration."}, nil)
	t.cleanupRuns = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hooklook_expiry_cleanup_runs_total", Help: "Expiry cleanup outcomes."}, []string{"result"})
	t.expired = prometheus.NewCounter(prometheus.CounterOpts{Name: "hooklook_expired_bins_deleted_total", Help: "Expired bins deleted."})
	t.sseConnections = prometheus.NewGauge(prometheus.GaugeOpts{Name: "hooklook_sse_connections", Help: "Open SSE streams."})
	t.sseEvents = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hooklook_sse_events_total", Help: "SSE opens, closes, and failures."}, []string{"event"})
	t.storage = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "hooklook_storage_bytes", Help: "SQLite and filesystem capacity in bytes."}, []string{"kind"})
	t.storageRatio = prometheus.NewGauge(prometheus.GaugeOpts{Name: "hooklook_storage_occupancy_ratio", Help: "Main SQLite allocation divided by configured budget."})
	t.storageAvailable = prometheus.NewGauge(prometheus.GaugeOpts{Name: "hooklook_storage_collection_available", Help: "Whether last storage collection succeeded."})
	t.activeBins = prometheus.NewGauge(prometheus.GaugeOpts{Name: "hooklook_active_bins", Help: "Active bins at last successful collection."})
	t.activeBinsAvailable = prometheus.NewGauge(prometheus.GaugeOpts{Name: "hooklook_active_bins_collection_available", Help: "Whether last active-bin collection succeeded."})
	t.nearLimit = prometheus.NewGauge(prometheus.GaugeOpts{Name: "hooklook_active_bins_near_limit", Help: "Active bins above 90 percent of count or body allowance."})
	t.pool = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "hooklook_db_pool_connections", Help: "Database pool connections."}, []string{"state"})
	t.poolWait = prometheus.NewCounter(prometheus.CounterOpts{Name: "hooklook_db_pool_wait_seconds_total", Help: "Cumulative time waiting for a SQLite connection."})
	t.registry.MustRegister(t.httpRequests, t.httpDuration, t.inFlight, t.operations, t.captures, t.dbDuration, t.dbOperations, t.dbErrors, t.cleanupDuration, t.cleanupRuns, t.expired, t.sseConnections, t.sseEvents, t.storage, t.storageRatio, t.storageAvailable, t.activeBins, t.activeBinsAvailable, t.nearLimit, t.pool, t.poolWait, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return t
}

func routeClass(r *http.Request) string {
	p := r.URL.Path
	switch {
	case p == "/":
		return "home"
	case p == "/health":
		return "health"
	case strings.HasPrefix(p, "/b/"):
		return "capture"
	case strings.HasSuffix(p, "/events") && strings.HasPrefix(p, "/api/bins/"):
		return "sse"
	case strings.HasPrefix(p, "/assets/") || strings.HasPrefix(p, "/fonts/") || p == "/favicon.svg":
		return "static"
	case strings.HasPrefix(p, "/admin/bins"):
		return "admin_bins"
	case strings.HasPrefix(p, "/admin/storage"):
		return "admin_storage"
	case strings.HasPrefix(p, "/api/bins/"):
		if strings.Contains(p, "/requests/") {
			return "api_request_detail"
		}
		if strings.HasSuffix(p, "/requests") {
			return "api_request_list"
		}
		if strings.HasSuffix(p, "/sharing") {
			return "api_sharing"
		}
		return "api_bin"
	case strings.HasPrefix(p, "/bins/"):
		return "bin_page"
	default:
		return "other"
	}
}
func methodClass(method string) string {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
		return method
	}
	return "OTHER"
}
func statusClass(status int) string {
	if status < 100 || status > 599 {
		return "other"
	}
	return string([]byte{byte('0' + status/100), byte('0' + status/10%10), byte('0' + status%10)})
}

type observedWriter struct {
	http.ResponseWriter
	status int
}

func (w *observedWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *observedWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}
func (w *observedWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func (w *observedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func observeHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := routeClass(r)
		method := methodClass(r.Method)
		started := time.Now()
		ow := &observedWriter{ResponseWriter: w}
		if route != "sse" {
			telemetry.inFlight.Inc()
			defer telemetry.inFlight.Dec()
		}
		next.ServeHTTP(ow, r)
		status := ow.status
		if status == 0 {
			status = http.StatusOK
		}
		telemetry.httpRequests.WithLabelValues(route, method, statusClass(status)).Inc()
		if route != "sse" {
			telemetry.httpDuration.WithLabelValues(route, method).Observe(time.Since(started).Seconds())
		}
		if route != "static" {
			slog.Info("http_request", "route", route, "method", method, "status", status, "duration_seconds", time.Since(started).Seconds())
		}
	})
}

func readiness(w http.ResponseWriter, r *http.Request) {
	if store == nil {
		http.Error(w, "unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()
	var n int
	if err := store.db.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
		http.Error(w, "unavailable", 503)
		return
	}
	w.WriteHeader(200)
	_, _ = w.Write([]byte("ready\n"))
}
func metricsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(telemetry.registry, promhttp.HandlerOpts{}))
	return mux
}

func collectTelemetry(ctx context.Context, s *Store) {
	c, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	var active, near int64
	err := s.db.QueryRowContext(c, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN total_body_bytes >= 90000000 OR (SELECT COUNT(*) FROM requests WHERE bin_code=bins.code) >= 450 THEN 1 ELSE 0 END),0) FROM bins WHERE expires_at > ?`, time.Now().Unix()).Scan(&active, &near)
	if err != nil {
		telemetry.activeBinsAvailable.Set(0)
	} else {
		telemetry.activeBins.Set(float64(active))
		telemetry.nearLimit.Set(float64(near))
		telemetry.activeBinsAvailable.Set(1)
	}
	capacity, err := s.storeCapacityContext(c)
	if err != nil {
		telemetry.storageAvailable.Set(0)
	} else {
		telemetry.storageAvailable.Set(1)
		telemetry.storage.WithLabelValues("budget").Set(float64(capacity.MaxBytes))
		telemetry.storage.WithLabelValues("allocated").Set(float64(capacity.DatabaseBytes))
		telemetry.storage.WithLabelValues("reusable").Set(float64(capacity.ReusableBytes))
		telemetry.storageRatio.Set(float64(capacity.DatabaseBytes) / float64(capacity.MaxBytes))
	}
	if fi, err := os.Stat(databasePath); err == nil {
		telemetry.storage.WithLabelValues("main_file").Set(float64(fi.Size()))
	}
	if fi, err := os.Stat(databasePath + "-wal"); err == nil {
		telemetry.storage.WithLabelValues("wal").Set(float64(fi.Size()))
	} else {
		telemetry.storage.WithLabelValues("wal").Set(0)
	}
	var stat syscall.Statfs_t
	if syscall.Statfs(".", &stat) == nil {
		telemetry.storage.WithLabelValues("filesystem_available").Set(float64(stat.Bavail) * float64(stat.Bsize))
	}
	pool := s.db.Stats()
	telemetry.pool.WithLabelValues("open").Set(float64(pool.OpenConnections))
	telemetry.pool.WithLabelValues("in_use").Set(float64(pool.InUse))
	telemetry.pool.WithLabelValues("idle").Set(float64(pool.Idle))
	telemetry.poolWait.Add(pool.WaitDuration.Seconds() - lastPoolWait)
	lastPoolWait = pool.WaitDuration.Seconds()
}

var lastPoolWait float64

func telemetryWorker(ctx context.Context, s *Store) {
	collectTelemetry(ctx, s)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collectTelemetry(ctx, s)
		}
	}
}
func dbKind(err error) string {
	if err == nil {
		return "none"
	}
	if err == sql.ErrNoRows {
		return "not_found"
	}
	if strings.Contains(strings.ToLower(err.Error()), "full") {
		return "full"
	}
	return "other"
}

func observeDBOperation(operation string, started time.Time, err error) {
	telemetry.dbDuration.WithLabelValues(operation).Observe(time.Since(started).Seconds())
	result := "success"
	if err != nil {
		result = "error"
		telemetry.dbErrors.WithLabelValues(operation, dbKind(err)).Inc()
	}
	telemetry.dbOperations.WithLabelValues(operation, result).Inc()
}

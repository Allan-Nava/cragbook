// Package metrics is cragbook's Prometheus adapter: the requests the API serves and how each
// import ended, on a registry of its own with the Go runtime's and the process's, for /metrics.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Allan-Nava/cragbook/internal/application/importing"
)

var _ importing.Observer = (*Metrics)(nil)

// Metrics records; Handler serves what it recorded in Prometheus' text format.
type Metrics struct {
	reg         *prometheus.Registry
	requests    *prometheus.CounterVec
	duration    *prometheus.HistogramVec
	imports     *prometheus.CounterVec
	lastRun     prometheus.Gauge
	lastSuccess prometheus.Gauge
	crags       prometheus.Gauge
	routes      prometheus.Gauge
	failed      prometheus.Gauge
}

// New registers cragbook's metrics, the build's version among them.
func New(version string) *Metrics {
	f := func(name, help string) prometheus.Gauge {
		return prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "cragbook", Name: name, Help: help})
	}
	m := &Metrics{
		reg: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: "cragbook", Name: "http_requests_total",
			Help: "Requests served, by method, route pattern and status."}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: "cragbook", Name: "http_request_duration_seconds",
			Help: "How long requests took, by method and route pattern.", Buckets: prometheus.DefBuckets}, []string{"method", "route"}),
		imports: prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: "cragbook", Name: "imports_total",
			Help: "Imports run, by result: ok, or error when the listing or the time ran out."}, []string{"result"}),
		lastRun:     f("import_last_run_timestamp_seconds", "When the last import ended, however it ended."),
		lastSuccess: f("import_last_success_timestamp_seconds", "When the last import that went through ended."),
		crags:       f("import_crags", "Crags the last import that went through saved."),
		routes:      f("import_routes", "Routes the last import that went through saved."),
		failed:      f("import_failed_crags", "Crags the last import that went through left as they were."),
	}
	build := prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "cragbook", Name: "build_info",
		Help: "Always 1, labelled with the build's version.", ConstLabels: prometheus.Labels{"version": version}})
	build.Set(1)
	m.reg.MustRegister(m.requests, m.duration, m.imports, m.lastRun, m.lastSuccess, m.crags, m.routes, m.failed, build,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// Served counts a request; route is the mux's pattern, never the path, so ids do not multiply
// the series.
func (m *Metrics) Served(method, route string, status int, d time.Duration) {
	m.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.duration.WithLabelValues(method, route).Observe(d.Seconds())
}

// Imported records how a run ended; the counts are the last run that went through.
func (m *Metrics) Imported(at time.Time, sum importing.Summary, err error) {
	m.lastRun.Set(float64(at.Unix()))
	if err != nil {
		m.imports.WithLabelValues("error").Inc()
		return
	}
	m.imports.WithLabelValues("ok").Inc()
	m.lastSuccess.Set(float64(at.Unix()))
	m.crags.Set(float64(sum.Crags))
	m.routes.Set(float64(sum.Routes))
	m.failed.Set(float64(len(sum.Failed)))
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{Registry: m.reg})
}

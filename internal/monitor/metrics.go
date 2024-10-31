package monitor

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricsCollector struct {
	requestDuration *prometheus.HistogramVec
	blockedRequests *prometheus.CounterVec
	successRequests *prometheus.CounterVec
}

func NewMetricsCollector() *MetricsCollector {
	m := &MetricsCollector{
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "shielder_request_duration_seconds",
				Help:    "Duration of requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"path"},
		),
		blockedRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shielder_blocked_requests_total",
				Help: "Total number of blocked requests",
			},
			[]string{"ip"},
		),
		successRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shielder_successful_requests_total",
				Help: "Total number of successful requests",
			},
			[]string{"ip"},
		),
	}

	return m
}

func (m *MetricsCollector) ObserveRequestDuration(path string, duration time.Duration) {
	m.requestDuration.WithLabelValues(path).Observe(duration.Seconds())
}

func (m *MetricsCollector) IncBlockedRequests(ip string) {
	m.blockedRequests.WithLabelValues(ip).Inc()
}

func (m *MetricsCollector) IncSuccessfulRequests(ip string) {
	m.successRequests.WithLabelValues(ip).Inc()
}

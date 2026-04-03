package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	VacanciesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hh_parser_vacancies_processed_total",
			Help: "Total number of processed vacancies",
		},
		[]string{"status"}, // success, error
	)
	HTTPRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hh_parser_http_requests_total",
			Help: "Total HTTP requests to HH API",
		},
		[]string{"method", "endpoint", "status"},
	)
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hh_parser_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
	ActiveWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hh_parser_active_workers",
			Help: "Current number of active workers",
		},
	)
)

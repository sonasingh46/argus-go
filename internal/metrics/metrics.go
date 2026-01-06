// Package metrics provides Prometheus metrics for ArgusGo.
// It tracks event ingestion, alert creation, and notification latencies
// to help identify performance bottlenecks and measure SLOs.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "argus"
)

// Event metrics track the ingestion pipeline.
var (
	// EventsReceivedTotal counts total events received by the API.
	EventsReceivedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "events_received_total",
			Help:      "Total number of events received by the ingest API",
		},
		[]string{"event_manager_id", "action"},
	)

	// EventsPublishedTotal counts events successfully published to the queue.
	EventsPublishedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "events_published_total",
			Help:      "Total number of events published to the message queue",
		},
		[]string{"event_manager_id"},
	)

	// EventsProcessedTotal counts events processed by the processor.
	EventsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "events_processed_total",
			Help:      "Total number of events processed",
		},
		[]string{"event_manager_id", "action", "result"},
	)

	// EventIngestLatency measures time from API receipt to queue publish.
	EventIngestLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "event_ingest_latency_seconds",
			Help:      "Time from event receipt to queue publish in seconds",
			Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// EventQueueLatency measures time spent in the queue.
	EventQueueLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "event_queue_latency_seconds",
			Help:      "Time event spent in the message queue in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
	)

	// EventProcessingLatency measures time to process a single event.
	EventProcessingLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "event_processing_latency_seconds",
			Help:      "Time to process a single event in seconds",
			Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)
)

// Alert metrics track alert lifecycle.
var (
	// AlertsCreatedTotal counts alerts created, labeled by type.
	AlertsCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "alerts_created_total",
			Help:      "Total number of alerts created",
		},
		[]string{"event_manager_id", "type"}, // type: parent, child
	)

	// AlertsResolvedTotal counts alerts resolved.
	AlertsResolvedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "alerts_resolved_total",
			Help:      "Total number of alerts resolved",
		},
		[]string{"event_manager_id", "type"},
	)

	// AlertCreationLatency measures end-to-end time from event ingestion to alert creation.
	// This is the key SLO metric for alert arrival time.
	AlertCreationLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "alert_creation_latency_seconds",
			Help:      "End-to-end latency from event ingestion to alert creation in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
	)

	// ActiveAlerts tracks the current number of active alerts.
	ActiveAlerts = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_alerts",
			Help:      "Current number of active alerts",
		},
		[]string{"event_manager_id", "type"},
	)

	// AlertGroupSize tracks the number of children per parent alert.
	AlertGroupSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "alert_group_size",
			Help:      "Number of child alerts per parent alert",
			Buckets:   []float64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000},
		},
	)
)

// Notification metrics track the notification pipeline.
var (
	// NotificationsSentTotal counts notifications sent.
	NotificationsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "notifications_sent_total",
			Help:      "Total number of notifications sent",
		},
		[]string{"event_manager_id", "status"}, // status: success, failure
	)

	// NotificationLatency measures time from alert creation to notification dispatch.
	// This is the key SLO metric for notification time.
	NotificationLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "notification_latency_seconds",
			Help:      "Time from alert state change to notification dispatch in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
	)

	// EndToEndLatency measures total time from event ingestion to notification sent.
	// This is the ultimate SLO metric combining alert arrival + notification time.
	EndToEndLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "end_to_end_latency_seconds",
			Help:      "Total time from event ingestion to notification sent in seconds",
			Buckets:   []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		},
	)
)

// Queue metrics track message queue health.
var (
	// QueueDepth tracks the current number of messages in the queue.
	QueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "queue_depth",
			Help:      "Current number of messages in the queue",
		},
	)

	// QueuePublishLatency measures time to publish a message to the queue.
	QueuePublishLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "queue_publish_latency_seconds",
			Help:      "Time to publish a message to the queue in seconds",
			Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
		},
	)
)

// Storage metrics track database and cache operations.
var (
	// StorageOperationLatency measures latency of storage operations.
	StorageOperationLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "storage_operation_latency_seconds",
			Help:      "Latency of storage operations in seconds",
			Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5},
		},
		[]string{"store", "operation"}, // store: postgres, redis; operation: read, write
	)

	// StorageOperationsTotal counts storage operations.
	StorageOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "storage_operations_total",
			Help:      "Total number of storage operations",
		},
		[]string{"store", "operation", "status"}, // status: success, failure
	)
)

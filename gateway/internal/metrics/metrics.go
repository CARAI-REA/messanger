package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	WSConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_ws_connections",
		Help: "Current WebSocket connections",
	})
	MessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_ws_messages_total",
		Help: "WebSocket messages by direction",
	}, []string{"direction"})
	DisconnectsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gateway_ws_disconnects_total",
		Help: "WebSocket disconnects",
	})
	FanoutErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gateway_fanout_errors_total",
		Help: "Realtime fanout errors",
	})
)

func init() {
	prometheus.MustRegister(WSConnections, MessagesTotal, DisconnectsTotal, FanoutErrorsTotal)
}

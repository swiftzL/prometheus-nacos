package nacos

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/discovery"
)

type NacosMertrics struct {
	rpcGetCount      prometheus.Counter
	metricRegisterer discovery.MetricRegisterer
}

func (n NacosMertrics) Register() error {
	return n.metricRegisterer.RegisterMetrics()
}

func (n NacosMertrics) Unregister() {
	n.metricRegisterer.UnregisterMetrics()
}

func NewNacosMertrics(reg prometheus.Registerer, rmi discovery.RefreshMetricsInstantiator) discovery.DiscovererMetrics {
	m := &NacosMertrics{
		rpcGetCount: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "nacos_get_count",
				Help: "The number of nacos RPC call get count.",
			},
		),
	}

	m.metricRegisterer = discovery.NewMetricRegisterer(reg, []prometheus.Collector{
		m.rpcGetCount,
	})
	return m
}

package sarama_metrics

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
)

type PrometheusConfig struct {
	logger          sarama.StdLogger
	namespace       string
	Registry        metrics.Registry // Registry to be exported
	subsystem       string
	promRegistry    prometheus.Registerer //Prometheus registry
	FlushInterval   time.Duration         //interval to update prom metrics
	gauges          map[string]*prometheus.GaugeVec
	filteredMetrics string
}

const (
	ForBrokerString = "_for_broker_"
	ForTopicString  = "_for_topic_"
)

var (
	BrokerReg = regexp.MustCompile(ForBrokerString)
	TopicReg  = regexp.MustCompile(ForTopicString)
)

func NewPrometheusProvider(logger sarama.StdLogger, r metrics.Registry, namespace string, subsystem string, promRegistry prometheus.Registerer, FlushInterval time.Duration, filteredMetrics string) *PrometheusConfig {
	return &PrometheusConfig{
		logger:          logger,
		namespace:       namespace,
		subsystem:       subsystem,
		Registry:        r,
		promRegistry:    promRegistry,
		FlushInterval:   FlushInterval,
		gauges:          make(map[string]*prometheus.GaugeVec),
		filteredMetrics: filteredMetrics,
	}
}

func (c *PrometheusConfig) flattenKey(key string) string {
	key = strings.Replace(key, " ", "_", -1)
	key = strings.Replace(key, ".", "_", -1)
	key = strings.Replace(key, "-", "_", -1)
	key = strings.Replace(key, "=", "_", -1)
	key = strings.Replace(key, "/", "_", -1)
	return key
}

func (c *PrometheusConfig) createKey(name string) string {
	return fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)
}

func (c *PrometheusConfig) gaugeFromNameAndValue(name string, suffix string, val float64) {
	name = c.flattenKey(name)
	var broker string
	if strings.Contains(name, ForBrokerString) {
		parts := BrokerReg.Split(name, -1)
		name = parts[0]
		broker = parts[1]
	}
	var topic string
	if strings.Contains(name, ForTopicString) {
		parts := TopicReg.Split(name, -1)
		name = parts[0]
		topic = parts[1]
	}
	name = name + "_" + suffix
	key := c.createKey(name)
	gv, ok := c.gauges[key]
	if !ok {
		gaugeVecs := []string{}
		if broker != "" {
			gaugeVecs = append(gaugeVecs, "broker")
		}
		if topic != "" {
			gaugeVecs = append(gaugeVecs, "topic")
		}
		gv = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: c.flattenKey(c.namespace),
			Subsystem: c.flattenKey(c.subsystem),
			Name:      c.flattenKey(name),
			Help:      name,
		}, gaugeVecs)
		c.promRegistry.Register(gv)
		c.gauges[key] = gv
	}
	vec := []string{}
	if broker != "" {
		vec = append(vec, broker)
	}
	if topic != "" {
		vec = append(vec, topic)
	}
	g := gv.WithLabelValues(vec...)
	g.Set(val)
}

func (c *PrometheusConfig) UpdatePrometheusMetrics() {
	for range time.Tick(c.FlushInterval) {
		c.UpdatePrometheusMetricsOnce()
	}
}

func (c *PrometheusConfig) UpdatePrometheusMetricsOnce() error {
	c.Registry.Each(func(name string, i interface{}) {
		if c.filteredMetrics == "" {
			return
		}
		match, _ := regexp.MatchString(c.filteredMetrics, name)
		if match {
			switch metric := i.(type) {
			// Sarama library only use Meter and Histogram metric type
			case metrics.Histogram:
				snapshot := metric.Snapshot()
				samples := snapshot.Sample().Values()
				var sum int64 = 0
				for _, v := range samples {
					sum += v
				}
				var avg float64 = 0
				if len(samples) > 0 {
					avg = float64(sum) / float64(len(samples))
					c.gaugeFromNameAndValue(name, "avg", avg)
				}
				c.gaugeFromNameAndValue(name, "count", float64(snapshot.Count()))
			case metrics.Meter:
				snapshot := metric.Snapshot()
				c.gaugeFromNameAndValue(name, "count", float64(snapshot.Count()))
			default:
				c.logger.Printf("Unsupported metric type: %s", metric)
			}
		}
	})
	return nil
}

# go-metrics-prometheus

This project is based on https://github.com/deathowl/go-metrics-prometheus to collect metrics from sarama kafka client library to prometheus.

However we modified the original code in the following ways:

1. Only collect histogram and Meter type metrics since sarama library is only issuing these two of metrics
2. For Histogram, we only extract count and avg data points since those two are most frequently used metrics and the other data points in histograms are difficult to extract and not precise when the sampling window changes
3. Samara library and prometheusmetrics code generates too many metrics.
    1. new metrics are created for each repetition of broker or topic:
        - batch_size_for_broker_1011
        - batch_size_for_broker_1012
        - compressed_bytes_for_topic_t1
        - compressed_bytes_for_topic_t2
    2. We extract the topic/broker name from the metric and move them to labels:
        - metric-name: batch-size, label: 1011
        - metric-name: compressed-bytes, label: t1
4. Add a metric name filter (can be expressed in regex) to only collect the metrics matching the name pattern
5. Rename the file as sarama_metrics.go to indicate that it only works with sarama metrics
  
# Usage

```
  import "github.com/slackhq/go-metrics-prometheus"
  import "github.com/prometheus/client_golang/prometheus"
  import "github.com/Shopify/sarama"

  metricsRegistry := metrics.NewRegistry()
  saramaMetricsClient := NewPrometheusProvider(samara.StdLogger, metricsRegistry, "murron", "sarama", prometheus.DefaultRegisterer, 10 * time.Second, "request_size")
  go saramaMetricsClient.UpdatePrometheusMetrics()
```

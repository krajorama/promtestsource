package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	//"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultPort = "5001"

type MetricType uint8

const (
	Gauge MetricType = iota
	Histogram
	FloatHistogram
)

func (v MetricType) String() string {
	switch v {
	case Gauge:
		return "gauge"
	case Histogram:
		return "histogram"
	case FloatHistogram:
		return "floathistogram"
	default:
		return "unknown"
	}
}

type Config struct {
	ListenAddress string
	MetricType    string
}

func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.ListenAddress, "bind", fmt.Sprintf(":%s", defaultPort), "Bind address")
	f.StringVar(&cfg.MetricType, "type", "gauge", "The type of metric to generate: gauge, histogram, floathistogram")
}

var metricTypes = map[string]MetricType{
	"gauge": Gauge,
	"histogram": Histogram,
	"floathistogram": FloatHistogram,
}

func Validate(cfg *Config) error {
	_, ok := metricTypes[cfg.MetricType]
	if !ok {
		return fmt.Errorf("unknown metric type %s", cfg.MetricType)
	}
	return nil
}

func main() {
	// Parse CLI flags.
	cfg := &Config{}
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	err := Validate(cfg)
	if err!=nil {
		fmt.Println(err)
		return
	}

	address, port := getAddressAndPort(cfg.ListenAddress)
	listenAddress := fmt.Sprintf("%s:%s", address, port)
	http.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	server := &http.Server{Addr: listenAddress, Handler: nil}
	defer server.Shutdown(context.Background())
	log.Printf("HTTP server on %s", listenAddress)

	go func() { log.Fatal(server.ListenAndServe()) }()

	labels := map[string]string{
		"address": address,
		"port": port,
		"generation": "20",
	}

	// mt := metricTypes[cfg.MetricType]
	// switch mt {
	// case Gauge:
	// 	handleGaugeInput(setupGauge(labels))
	// case Histogram:
	// 	handleHistogramInput(setupHistogram(labels))
	// default:
	// 	panic(fmt.Sprint("Not implemented for ", mt))
	// }

	// Start count from 100000
	h := setupHistogram(labels)
	for i := 0; i < 100000; i++ {
		h.Observe(1.0)
	}

	// 10 times per second
	tick := time.NewTicker(100*time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			h.Observe(1.0)
		}
	}
}

// getAddressAndPort always defines a non empty address and port
//
// The Go http server can use empty to mean any, but we want
// something meaningful in the metric labels.
func getAddressAndPort(listenAddress string) (string, string) {
	address, port, error := net.SplitHostPort(listenAddress)
	if error != nil {
		log.Fatal(error)
	}
	if address == "" {
		address = "0.0.0.0"
	}
	if port == "" {
		port = defaultPort
	}

	return address, port
}

// func setupGauge(labels map[string]string) prometheus.Gauge {
// 	gauge := prometheus.NewGauge(
// 		prometheus.GaugeOpts{
// 			Namespace: "golang",
// 			Name:      "manual_gauge",
// 			Help:      "This is my manual gauge",
// 			ConstLabels: labels,
// 		})
// 	prometheus.MustRegister(gauge)
// 	return gauge
// }

// func handleGaugeInput(gauge prometheus.Gauge) {
// 	currentValue := 0.0
// 	gauge.Set(currentValue)
// 	scanner := bufio.NewScanner(os.Stdin)
// 	scan := func() bool {
// 		fmt.Printf("Set metric to x or add with +x (current: %v): ", currentValue)
// 		return scanner.Scan()
// 	}
// 	for scan() {
// 		textToParse := scanner.Text()
// 		isAdd := false
// 		if strings.HasPrefix(textToParse, "+") {
// 			isAdd = true
// 			textToParse = strings.TrimPrefix(textToParse, "+")
// 		}
// 		newValue, error := strconv.ParseFloat(textToParse, 64)
// 		if error != nil {
// 			continue
// 		}
// 		if isAdd {
// 			currentValue += newValue
// 		} else {
// 			currentValue = newValue
// 		}
// 		gauge.Set(currentValue)
// 	}
// }

func setupHistogram(labels map[string]string) prometheus.Histogram {
	histogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "golang",
			Name: "manual_histogram",
			Help: "This is a histogram with manually selected parameters",
			ConstLabels: labels,
			NativeHistogramBucketFactor: 1.1,
			NativeHistogramMaxBucketNumber: 100,
			NativeHistogramMinResetDuration: 1*time.Hour,
			Buckets: prometheus.DefBuckets,
	})
	prometheus.MustRegister(histogram)
	//histogram.(prometheus.ExemplarObserver).ObserveWithExemplar(1.0, prometheus.Labels{"foo": "bar1"})
	// time.Sleep(1*time.Second)
	// histogram.(prometheus.ExemplarObserver).ObserveWithExemplar(4.0, prometheus.Labels{"foo": "bar2"})
	return histogram
}

func handleHistogramInput(histogram prometheus.Histogram) {
	scanner := bufio.NewScanner(os.Stdin)
	scan := func() bool {
		fmt.Printf("Make an observation:")
		return scanner.Scan()
	}
	for scan() {
		newValue, error := strconv.ParseFloat(scanner.Text(), 64)
		//histogram.Observe(newValue)
		histogram.(prometheus.ExemplarObserver).ObserveWithExemplar(newValue, prometheus.Labels{"foo": "bar3"})
		if error != nil {
			continue
		}
	}
}

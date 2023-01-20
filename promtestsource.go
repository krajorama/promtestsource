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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultPort = "5001"

type Config struct {
	ListenAddress string
	MetricType    string
}

func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.ListenAddress, "bind", fmt.Sprintf(":%s", defaultPort), "Bind address")
	f.StringVar(&cfg.MetricType, "type", "gauge", "The type of metric to generate")
}

func Validate(cfg *Config) error {
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
	log.Printf("HTTP server on %s", listenAddress)

	labels := map[string]string{
		"address": address,
		"port": port,
	}
	gauge := setupGauge(labels)

	http.Handle("/metrics", promhttp.Handler())
	server := &http.Server{Addr: listenAddress, Handler: nil}
	defer server.Shutdown(context.Background())

	go func() { log.Fatal(server.ListenAndServe()) }()

	handleInput(gauge)
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

func setupGauge(labels map[string]string) prometheus.Gauge {
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "golang",
			Name:      "manual_gauge",
			Help:      "This is my manual gauge",
			ConstLabels: labels,
		})
	prometheus.MustRegister(gauge)
	return gauge
}

func handleInput(gauge prometheus.Gauge) {
	currentValue := 0.0
	gauge.Set(currentValue)
	scanner := bufio.NewScanner(os.Stdin)
	scan := func() bool {
		fmt.Printf("Set metric (current: %v): ", currentValue)
		return scanner.Scan()
	}
	for scan() {
		newValue, error := strconv.ParseFloat(scanner.Text(), 64)
		if error != nil {
			continue
		}
		currentValue = newValue
		gauge.Set(currentValue)
	}
}

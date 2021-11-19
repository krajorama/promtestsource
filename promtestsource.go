package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	listenAddress := ":5001"
	if len(os.Args) > 1 {
		listenAddress = os.Args[1]
	} else {
		log.Println("You can specify different listen address on the command line, e.g. go run promtestsource.go :5002")
	}
	address, port := getAddressAndPort(listenAddress)
	listenAddress = fmt.Sprintf("%s:%s", address, port)
	log.Printf("HTTP server on %s", listenAddress)

	gauge := setupGauge(address, port)

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
		port = "80"
	}

	return address, port
}

func setupGauge(address, port string) prometheus.Gauge {
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "golang",
			Name:      "manual_gauge",
			Help:      "This is my manual gauge",
			ConstLabels: map[string]string{
				"address": address,
				"port":    port,
			},
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

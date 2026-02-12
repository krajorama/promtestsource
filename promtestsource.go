package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultPort = "5001"

type MetricType uint8

const (
	Gauge MetricType = iota
	Histogram
	Counter
)

type HistogramType uint8

const (
	ClassicHistogram = iota
	NativeHistogramStandardBuckets
	NativeHistogramCustomBuckets
)

func (v MetricType) String() string {
	switch v {
	case Counter:
		return "counter"
	case Gauge:
		return "gauge"
	case Histogram:
		return "histogram"
	default:
		return "unknown"
	}
}

type Config struct {
	ListenAddress string
	MetricType    string
	HistogramType string
	Username      string
	Password      string
}

func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.ListenAddress, "bind", fmt.Sprintf(":%s", defaultPort), "Bind address")
	f.StringVar(&cfg.MetricType, "type", "gauge", "The type of metric to generate: counter, gauge, histogram")
	f.StringVar(&cfg.HistogramType, "histogram-type", "classic", "Type of histogram, comma separated: classic, native")
	f.StringVar(&cfg.Username, "username", "", "Basic auth username")
	f.StringVar(&cfg.Password, "password", "", "Basic auth password")
}

var metricTypes = map[string]MetricType{
	"counter": Counter,
	"gauge": Gauge,
	"histogram": Histogram,
}

var histogramTypes = map[string]HistogramType {
	"classic": ClassicHistogram,
	"native": NativeHistogramStandardBuckets,
}

func Validate(cfg *Config) error {
	if _, ok := metricTypes[cfg.MetricType]; !ok {
		return fmt.Errorf("unknown metric type %s", cfg.MetricType)
	}

	hTypes := strings.Split(cfg.HistogramType, ",")
	if len(hTypes) == 0 {
		return fmt.Errorf("histogram type needs to be specified")
	}
	for _, t := range hTypes {
		if _, ok := histogramTypes[t]; !ok {
			return fmt.Errorf("unknown histogram type %s", t)
		}
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
	handler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		EnableOpenMetrics:                   true,
		EnableOpenMetricsTextCreatedSamples: true,
	})

	if len(cfg.Username) > 0 && len(cfg.Password) > 0 {
		app := application{
			username: cfg.Username,
			password: cfg.Password,
		}
		handler = app.basicAuth(handler.ServeHTTP)
	}

	http.Handle("/metrics", handler)
	server := &http.Server{Addr: listenAddress, Handler: nil}
	defer server.Shutdown(context.Background())
	log.Printf("HTTP server on %s", listenAddress)

	go func() { log.Fatal(server.ListenAndServe()) }()

	labels := map[string]string{
		"address": address,
		"port": port,
	}

	mt := metricTypes[cfg.MetricType]
	switch mt {
	case Counter:
		handleCounterInput(setupCounter(labels))
	case Gauge:
		handleGaugeInput(setupGauge(labels))
	case Histogram:
		handleHistogramInput(setupHistogram(labels, cfg))
	default:
		panic(fmt.Sprint("Not implemented for ", mt))
	}

}

type application struct {
	username string
	password string
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

func setupCounter(labels map[string]string) prometheus.Counter {
	counter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "golang",
			Name: "manual_counter_total",
			Help: "This is a manual counter",
			ConstLabels: labels,
		},
	)
	prometheus.MustRegister(counter)
	return counter
}

func handleCounterInput(counter prometheus.Counter) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	fmt.Printf("Counter will be incremented every second.\n")
	for range ticker.C {
		counter.Inc()
	}
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

func handleGaugeInput(gauge prometheus.Gauge) {
	currentValue := 0.0
	gauge.Set(currentValue)
	scanner := bufio.NewScanner(os.Stdin)
	scan := func() bool {
		fmt.Printf("Set metric to x or add with +x (current: %v): ", currentValue)
		return scanner.Scan()
	}
	for scan() {
		textToParse := scanner.Text()
		isAdd := false
		if strings.HasPrefix(textToParse, "+") {
			isAdd = true
			textToParse = strings.TrimPrefix(textToParse, "+")
		}
		newValue, error := strconv.ParseFloat(textToParse, 64)
		if error != nil {
			continue
		}
		if isAdd {
			currentValue += newValue
		} else {
			currentValue = newValue
		}
		gauge.Set(currentValue)
	}
}

func setupHistogram(labels map[string]string, cfg *Config) prometheus.Histogram {
	opts := prometheus.HistogramOpts{
			Namespace: "http",
			Name: "request_seconds",
			Help: "This is a histogram with manually selected parameters",
			ConstLabels: labels,
	}
	htTypes  := strings.Split(cfg.HistogramType, ",")
	for _, t := range htTypes {
		hType := histogramTypes[t]
		switch hType {
		case ClassicHistogram:
			opts.Buckets = prometheus.DefBuckets
		case NativeHistogramStandardBuckets:
			opts.NativeHistogramBucketFactor = 1.1
			opts.NativeHistogramMaxBucketNumber = 100
			opts.NativeHistogramMinResetDuration = 1*time.Hour
		}
	}

	histogram := prometheus.NewHistogram(opts)

	// for i := 0;i<20;i++ {
	// 	if i<10 {
	// 		histogram.Observe(0)
	// 	} else {
	// 		histogram.Observe(2)
	// 	}
	// }

	prometheus.MustRegister(histogram)
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
		if error != nil {
			continue
		}
		fmt.Printf("Observed %v\n", newValue)
		histogram.Observe(newValue)
	}
}

func (app *application) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(app.username))
			expectedPasswordHash := sha256.Sum256([]byte(app.password))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// Command telemetry collects node metrics and exposes them via Prometheus.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aqstack/sentinel/pkg/collector"
	"github.com/aqstack/sentinel/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var (
		nodeName   = flag.String("node", "", "Node name (default: hostname)")
		listenAddr = flag.String("listen", ":9100", "Prometheus metrics listen address")
		interval   = flag.Duration("interval", time.Second, "Metrics collection interval")
		disk       = flag.String("disk", "", "Primary disk device to monitor (auto-detected if empty)")
		iface      = flag.String("iface", "", "Network interface to monitor (auto-detected if empty)")
	)
	flag.Parse()

	// Get node name
	if *nodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("Failed to get hostname: %v", err)
		}
		*nodeName = hostname
	}

	log.Printf("Starting AEOP telemetry collector on node %s", *nodeName)

	// Create collector with options
	opts := []collector.Option{}
	if *disk != "" {
		opts = append(opts, collector.WithDisk(*disk))
	}
	if *iface != "" {
		opts = append(opts, collector.WithNetworkInterface(*iface))
	}

	c, err := collector.New(*nodeName, opts...)
	if err != nil {
		log.Fatalf("Failed to create collector: %v", err)
	}

	// Create metrics exporter
	exporter := metrics.NewExporter(*nodeName)
	reg := prometheus.NewRegistry()
	if err := exporter.Register(reg); err != nil {
		log.Fatalf("Failed to register metrics: %v", err)
	}

	// Start collection loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m, err := c.Collect(ctx)
				if err != nil {
					log.Printf("Collection error: %v", err)
					continue
				}
				exporter.Update(m)

				if len(m.Errors) > 0 {
					log.Printf("Partial collection errors: %v", m.Errors)
				}
			}
		}
	}()

	// Start HTTP server for Prometheus
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	server := &http.Server{
		Addr:    *listenAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("Serving metrics on %s/metrics", *listenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)
}

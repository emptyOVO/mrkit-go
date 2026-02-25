package master

import (
	"context"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func resolveMetricsAddr(masterAddr string) string {
	if raw := strings.TrimSpace(os.Getenv("MR_METRICS_ADDR")); raw != "" {
		return raw
	}

	addr := strings.TrimSpace(masterAddr)
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return ":11000"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return ":11000"
	}
	metricsPort := port + 1000
	if host == "" || host == "0.0.0.0" || host == "::" {
		return ":" + strconv.Itoa(metricsPort)
	}
	return net.JoinHostPort(host, strconv.Itoa(metricsPort))
}

func startMetricsServer(ms *Master, masterAddr string) func() {
	if ms == nil {
		return func() {}
	}
	metricsAddr := resolveMetricsAddr(masterAddr)
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(ms.metricsSnapshot()))
	})

	httpServer := &http.Server{
		Addr:              metricsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		log.Infof("[Master] metrics endpoint listening on %s/metrics", metricsAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warnf("[Master] metrics endpoint stopped: %v", err)
		}
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Warnf("[Master] metrics endpoint shutdown failed: %v", err)
		}
	}
}

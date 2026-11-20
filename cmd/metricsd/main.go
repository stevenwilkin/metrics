package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/joho/godotenv/autoload"
)

var (
	metrics map[int64]*metric
	conns   map[*websocket.Conn]bool
	mi      sync.Mutex
	m       sync.Mutex
)

func updateTicker(name string, t ticker) {
	mi.Lock()
	defer mi.Unlock()

	for _, item := range metrics {
		if item.Name == name {
			item.Price = t.MarkPrice
			item.Index = t.IndexPrice
			item.Funding = t.CurrentFunding
			return
		}
	}
}

func initMetrics() {
	metrics = map[int64]*metric{}

	streamTickers(updateTicker)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)

		for {
			timestamps := map[int64]bool{}
			added := []string{}
			removed := []string{}
			instruments := getInstruments()
			mi.Lock()

			for _, i := range instruments {
				timestamps[i.ExpirationTimestamp] = true

				if _, ok := metrics[i.ExpirationTimestamp]; !ok {
					metrics[i.ExpirationTimestamp] = &metric{
						Name:      i.InstrumentName,
						Timestamp: i.ExpirationTimestamp}
					added = append(added, i.InstrumentName)
				}
			}

			// prune expired instruments
			for timestamp, item := range metrics {
				if _, ok := timestamps[timestamp]; !ok {
					removed = append(removed, item.Name)
					delete(metrics, timestamp)
				}
			}

			mi.Unlock()

			unsubscribeTickers(removed)
			subscribeTickers(added)

			<-ticker.C
		}
	}()
}

func sendState(c *websocket.Conn) error {
	mi.Lock()

	if len(metrics) == 0 {
		mi.Unlock()
		return nil
	}

	results := make([]metric, len(metrics))
	timestamps := make([]int, len(metrics))
	i := 0

	for _, metric := range metrics {
		timestamps[i] = int(metric.Timestamp)
		i++
	}

	sort.Ints(timestamps)

	// perp has longest duration, add it first
	results[0] = *metrics[int64(timestamps[len(timestamps)-1])]

	for i := 1; i < len(timestamps); i++ {
		results[i] = *metrics[int64(timestamps[i-1])]
	}

	mi.Unlock()

	if err := c.WriteJSON(results); err != nil {
		return err
	}

	return nil
}

func handleWs(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Accepting connection")

	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		}}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn(err.Error())
		return
	}

	sendState(c)

	m.Lock()
	conns[c] = true
	m.Unlock()
}

func initWS() {
	conns = map[*websocket.Conn]bool{}

	http.HandleFunc("/ws", handleWs)

	port := "8080"
	if wwwPort := os.Getenv("METRICSD_PORT"); len(wwwPort) > 0 {
		port = wwwPort
	}

	go func() {
		slog.Info(fmt.Sprintf("Listening on 0.0.0.0:%s", port))
		slog.Error(http.ListenAndServe(fmt.Sprintf(":%s", port), nil).Error())
	}()

	go func() {
		ticker := time.NewTicker(1 * time.Second)

		for {
			for c, _ := range conns {
				if err := sendState(c); err != nil {
					slog.Debug(err.Error())
					m.Lock()
					delete(conns, c)
					m.Unlock()
				}
			}

			<-ticker.C
		}
	}()
}

func trapSigInt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-c
}

func initLog() {
	logLevel := slog.LevelInfo
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		if err := logLevel.UnmarshalText([]byte(lvl)); err != nil {
			slog.Warn(fmt.Sprintf("Unknown log level %q, defaulting to INFO", lvl))
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))
}

func main() {
	initLog()

	slog.Info("Starting")

	initMetrics()
	initWS()
	trapSigInt()

	slog.Info("Stopping")
}

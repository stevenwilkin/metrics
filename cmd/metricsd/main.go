package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/joho/godotenv/autoload"
)

var (
	conns map[*websocket.Conn]bool
	m     sync.Mutex
)

func sendState(c *websocket.Conn) error {
	sm := stateMessage{Now: time.Now().UnixMilli()}

	if err := c.WriteJSON(sm); err != nil {
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

func main() {
	slog.Info("Starting")

	initWS()
	trapSigInt()

	slog.Info("Stopping")
}

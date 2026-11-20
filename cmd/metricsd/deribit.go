package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const deribitHost = "www.deribit.com"

var (
	wsConn   *websocket.Conn
	channels = map[string]bool{} // active ticker subscriptions
	wsMu     sync.Mutex          // guards wsConn writes and channels
)

func getJSON(path string, params url.Values, response interface{}) error {
	u := url.URL{
		Scheme:   "https",
		Host:     deribitHost,
		Path:     path,
		RawQuery: params.Encode()}

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Returned HTTP %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	return nil
}

func getInstruments() []instrument {
	var response instrumentsResponse
	err := getJSON(
		"/api/v2/public/get_instruments",
		url.Values{"currency": {"BTC"}, "kind": {"future"}},
		&response)
	if err != nil {
		return []instrument{}
	}

	return response.Result
}

func tickerChannel(name string) string {
	return fmt.Sprintf("ticker.%s.100ms", name)
}

// writeWS sends a request over the current connection, if any.
// The caller must hold wsMu.
func writeWS(method string, params map[string]interface{}) error {
	if wsConn == nil {
		return nil // will be (re)subscribed on connect
	}

	slog.Debug("Sending request", "method", method, "params", params)
	return wsConn.WriteJSON(requestMessage{
		JsonRpc: "2.0",
		Method:  method,
		Params:  params})
}

func subscribeTickers(names []string) {
	if len(names) == 0 {
		return
	}

	chs := make([]string, len(names))
	for i, name := range names {
		chs[i] = tickerChannel(name)
	}

	wsMu.Lock()
	defer wsMu.Unlock()

	for _, ch := range chs {
		channels[ch] = true
	}

	if err := writeWS("/public/subscribe", map[string]interface{}{"channels": chs}); err != nil {
		slog.Warn(err.Error())
	}
}

func unsubscribeTickers(names []string) {
	if len(names) == 0 {
		return
	}

	chs := make([]string, len(names))
	for i, name := range names {
		chs[i] = tickerChannel(name)
	}

	wsMu.Lock()
	defer wsMu.Unlock()

	for _, ch := range chs {
		delete(channels, ch)
	}

	if err := writeWS("/public/unsubscribe", map[string]interface{}{"channels": chs}); err != nil {
		slog.Warn(err.Error())
	}
}

func connectWS() (*websocket.Conn, error) {
	socketUrl := url.URL{Scheme: "wss", Host: deribitHost, Path: "/ws/api/v2"}

	c, _, err := websocket.DefaultDialer.Dial(socketUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	wsMu.Lock()
	defer wsMu.Unlock()

	wsConn = c

	chs := make([]string, 0, len(channels))
	for ch := range channels {
		chs = append(chs, ch)
	}

	if len(chs) > 0 {
		if err := writeWS("/public/subscribe", map[string]interface{}{"channels": chs}); err != nil {
			wsConn = nil
			c.Close()
			return nil, err
		}
	}

	return c, nil
}

func heartbeat(c *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			wsMu.Lock()
			err := c.WriteJSON(requestMessage{JsonRpc: "2.0", Method: "/public/test"})
			wsMu.Unlock()

			if err != nil {
				return
			}
		}
	}
}

func readTickers(c *websocket.Conn, onTicker func(string, ticker)) error {
	for {
		// heartbeat responses arrive every 10s so this only expires on a dead connection
		c.SetReadDeadline(time.Now().Add(30 * time.Second))

		_, message, err := c.ReadMessage()
		if err != nil {
			return err
		}

		var msg tickerMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			slog.Debug(err.Error())
			continue
		}

		if msg.Method != "subscription" || msg.Params.Data.InstrumentName == "" {
			continue
		}

		onTicker(msg.Params.Data.InstrumentName, msg.Params.Data.ticker)
	}
}

// streamTickers maintains a websocket connection to Deribit, passing ticker
// updates for all subscribed instruments to onTicker. Reconnects on failure.
func streamTickers(onTicker func(string, ticker)) {
	go func() {
		for {
			c, err := connectWS()
			if err != nil {
				slog.Warn(err.Error())
				time.Sleep(5 * time.Second)
				continue
			}

			slog.Info("Connected to Deribit")

			done := make(chan struct{})
			go heartbeat(c, done)

			err = readTickers(c, onTicker)
			slog.Warn(fmt.Sprintf("Deribit connection lost: %s", err))

			close(done)
			wsMu.Lock()
			wsConn = nil
			wsMu.Unlock()
			c.Close()

			time.Sleep(1 * time.Second)
		}
	}()
}

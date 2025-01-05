package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/gorilla/websocket"
	_ "github.com/joho/godotenv/autoload"
)

type stateMessage []struct {
	Name      string  `json:"name"`
	Timestamp int64   `json:"timestamp"`
	Price     float64 `json:"price"`
	Index     float64 `json:"index"`
	Funding   float64 `json:"funding"`
}

func main() {
	host := os.Getenv("METRICSD_HOST")
	port := os.Getenv("METRICSD_PORT")

	u := url.URL{
		Scheme: "ws",
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   "/ws"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			fmt.Println(err.Error())
			c.Close()
			os.Exit(1)
		}

		var sm stateMessage
		json.Unmarshal(message, &sm)

		fmt.Println("\033[2J\033[H\033[?25l") // clear screen, move cursor to top of screen, hide cursor
		fmt.Println(sm)
	}
}

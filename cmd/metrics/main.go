package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

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

func tenor(ms int64) string {
	days := ms / (1000 * 60 * 60 * 24)
	hours := (ms / (1000 * 60 * 60)) % 24
	minutes := (ms / (1000 * 60)) % 60

	return fmt.Sprintf("%3dd %2dh %2dm", days, hours, minutes)
}

// render writes a frame using Synchronized Output (DEC mode 2026) so the
// terminal paints it atomically, avoiding flicker between clear and redraw
func render(f func(*strings.Builder)) {
	var sb strings.Builder
	sb.WriteString("\033[?2026h")              // begin synchronized update
	sb.WriteString("\033[2J\033[H\033[?25l\n") // clear screen, move cursor to top of screen, hide cursor
	f(&sb)
	sb.WriteString("\033[?2026l") // end synchronized update
	os.Stdout.WriteString(sb.String())
}

func display(sm stateMessage) {
	render(func(sb *strings.Builder) {
		for _, i := range sm {
			msToExpiration := i.Timestamp - time.Now().UnixMilli()

			premium := i.Price - i.Index
			yield := premium / i.Index
			annualisedYield := yield / (float64(msToExpiration) / (1000 * 60 * 60 * 24 * 365))

			if i.Name == "BTC-PERPETUAL" {
				fmt.Fprintf(sb, "  %-13s %8.2f           %9.6f%%\n", i.Name, premium, i.Funding*100)
			} else {
				fmt.Fprintf(sb, "  %-13s %8.2f %6.2f%% %s\n", i.Name, premium, annualisedYield*100, tenor(msToExpiration))
			}
		}
	})
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

		display(sm)
	}
}

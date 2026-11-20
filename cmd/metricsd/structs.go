package main

type metric struct {
	Name      string  `json:"name"`
	Timestamp int64   `json:"timestamp"`
	Price     float64 `json:"price"`
	Index     float64 `json:"index"`
	Funding   float64 `json:"funding"`
}

type instrument struct {
	InstrumentName      string `json:"instrument_name"`
	ExpirationTimestamp int64  `json:"expiration_timestamp"`
}

type instrumentsResponse struct {
	Result []instrument `json:"result"`
}

type ticker struct {
	MarkPrice      float64 `json:"mark_price"`
	IndexPrice     float64 `json:"index_price"`
	CurrentFunding float64 `json:"current_funding"`
}

type requestMessage struct {
	JsonRpc string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type tickerMessage struct {
	Method string `json:"method"`
	Params struct {
		Channel string `json:"channel"`
		Data    struct {
			InstrumentName string `json:"instrument_name"`
			ticker
		} `json:"data"`
	} `json:"params"`
}

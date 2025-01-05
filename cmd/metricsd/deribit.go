package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

func getJSON(path string, params url.Values, response interface{}) error {
	u := url.URL{
		Scheme:   "https",
		Host:     "www.deribit.com",
		Path:     path,
		RawQuery: params.Encode()}

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	json.Unmarshal(body, &response)

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

func getTicker(name string) ticker {
	var response tickerResponse
	err := getJSON(
		"/api/v2/public/ticker",
		url.Values{"instrument_name": {name}},
		&response)
	if err != nil {
		return ticker{}
	}

	return response.Result
}

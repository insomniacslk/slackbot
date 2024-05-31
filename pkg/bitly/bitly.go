package bitly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

/* Old API v3, deprecated and disabled on the 1st of March 2020.

import (
	gobitly "github.com/zpnk/go-bitly"
)

// Bitly returns a shortened URL using the Bit.ly URL shortener API v3.
func Bitly(apiKey string, longURL string) (string, error) {
	c := gobitly.New(apiKey)
	l, err := c.Links.Shorten(longURL)
	if err != nil {
		return "", err
	}
	return l.URL, nil
}
*/

// References is part of the Bitly4Response struct
type References struct {
	Group string `json:"group"`
}

// Bitly4Response maps to the JSON response of a Bitly API v4 shorten request.
type Bitly4Response struct {
	CreatedAt      string     `json:"created_at"`
	ID             string     `json:"id"`
	Link           string     `json:"link"`
	CustomBitlinks []string   `json:"custom_bitlinks"`
	LongURL        string     `json:"long_url"`
	Archived       bool       `json:"archived"`
	Tags           []string   `json:"tags"`
	Deeplinks      []string   `json:"deeplinks"`
	References     References `json:"references"`
}

// Bitly returns a shortened URL using the Bit.ly URL shortener API v4.
func Bitly(apiToken, longURL string) (string, error) {
	// minimal payload to run this request
	type bitlyPayload struct {
		LongURL string `json:"long_url"`
	}
	bp := bitlyPayload{LongURL: longURL}
	payload, err := json.Marshal(bp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal bitly v4 payload: %w", err)
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", "https://api-ssl.bitly.com/v4/shorten", bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close HTTP response body: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("HTTP status code is not 200 OK, got %d", resp.StatusCode)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read HTTP response body: %w", err)
	}
	blr := Bitly4Response{}
	if err := json.Unmarshal(data, &blr); err != nil {
		return "", fmt.Errorf("Failed to deserialize HTTP JSON response from Bitly: %w", err)
	}
	return blr.Link, nil
}

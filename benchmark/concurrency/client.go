package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *HTTPClient) GET(endpoint string) (*http.Response, error) {
	url := c.baseURL + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")

	return c.client.Do(req)
}

func (c *HTTPClient) POST(endpoint string, body interface{}) (*http.Response, error) {
	url := c.baseURL + endpoint
	
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")

	return c.client.Do(req)
}

func UnmarshalBody(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	
	return json.Unmarshal(body, v)
}

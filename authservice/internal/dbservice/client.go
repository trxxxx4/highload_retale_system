package dbservice

import (
	"auth/internal/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
	}
}

func NewClient(
	baseURL string,
	httpClient *http.Client,
) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *Client) AddUser(
	ctx context.Context,
	dto models.RegisterResponse,
) error {
	data, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("marshal user DTO: %w", err)
	}

	path := c.baseURL + "/internal/users"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		path,
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request to db-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"db-service returned status: %d",
			resp.StatusCode,
		)
	}

	return nil
}

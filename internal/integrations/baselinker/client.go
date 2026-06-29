package baselinker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

type Client struct {
	apiURL string
	token  string
	client *http.Client
}

func NewClient(apiURL, token string) *Client {
	return &Client{
		apiURL: apiURL,
		token:  token,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}
func (c *Client) GetOrders(ctx context.Context, dateConfirmedFrom int64) (*models.GetOrdersResponse, error) {
	log.Printf("📥 BaseLinker getOrders request started (date_confirmed_from=%d)", dateConfirmedFrom)

	if c.apiURL == "" {
		return nil, fmt.Errorf("BaseLinker API URL is not configured")
	}
	if c.token == "" {
		return nil, fmt.Errorf("BaseLinker token is not configured. Set BASELINKER_TOKEN in the backend .env and restart the server")
	}

	params := map[string]interface{}{
		"date_confirmed_from":    dateConfirmedFrom,
		"get_unconfirmed_orders": false, // confirmed orders only
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	data := url.Values{}
	data.Set("method", "getOrders")
	data.Set("parameters", string(paramsJSON))

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.apiURL,
		bytes.NewBufferString(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-BLToken", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"API returned status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	var result models.GetOrdersResponse

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.Status != "SUCCESS" {
		return nil, fmt.Errorf("API returned status: %s", result.Status)
	}

	log.Printf("✅ BaseLinker getOrders succeeded: %d orders returned", len(result.Orders))

	return &result, nil
}

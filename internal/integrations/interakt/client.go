package interakt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://api.interakt.ai"
const defaultCountryCode = "+91"
const defaultTemplateName = "amzmrclearorderconfirmation_v2"
const defaultTemplateLanguageCode = "en"

var nonDigitPattern = regexp.MustCompile(`[^\d]`)

type Config struct {
	BaseURL      string
	APIKey       string
	Enabled      bool
	Mode         string
	TestNumber   string
	TemplateName string
}

type Client struct {
	baseURL             string
	apiKey              string
	testNumber          string
	httpClient          *http.Client
	mu                  sync.RWMutex
	runtimeMode         string
	runtimeOn           bool
	runtimeTemplateName string
}

type SendOrderMessageRequest struct {
	CustomerName string
	OrderID      string
	OrderDetails string
	PhoneNumber  string
	CallbackData string
}

type messagePayload struct {
	CountryCode  string          `json:"countryCode"`
	PhoneNumber  string          `json:"phoneNumber"`
	CallbackData string          `json:"callbackData,omitempty"`
	Type         string          `json:"type"`
	Template     templatePayload `json:"template"`
}

type templatePayload struct {
	Name         string   `json:"name"`
	LanguageCode string   `json:"languageCode"`
	BodyValues   []string `json:"bodyValues"`
}

type messageResponse struct {
	Result  bool   `json:"result"`
	Message string `json:"message"`
	ID      string `json:"id"`
}

func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	client := &Client{
		baseURL:             baseURL,
		apiKey:              strings.TrimSpace(cfg.APIKey),
		testNumber:          strings.TrimSpace(cfg.TestNumber),
		httpClient:          &http.Client{Timeout: 30 * time.Second},
		runtimeOn:           cfg.Enabled,
		runtimeMode:         normalizeMode(cfg.Mode),
		runtimeTemplateName: normalizeTemplateName(cfg.TemplateName),
	}

	if !client.runtimeOn {
		log.Printf("ℹ️  Interakt initialized in disabled mode (mode=%s, template=%s)", client.runtimeMode, client.runtimeTemplateName)
		return client, nil
	}
	if client.apiKey == "" {
		log.Printf("⚠️  Interakt enabled but API key is missing (mode=%s, template=%s)", client.runtimeMode, client.runtimeTemplateName)
		return client, fmt.Errorf("INTERAKT_API_KEY is required when INTERAKT_ENABLED=true")
	}
	if client.runtimeMode == "test" && client.testNumber == "" {
		log.Printf("⚠️  Interakt test mode enabled but test number is missing (template=%s)", client.runtimeTemplateName)
		return client, fmt.Errorf("INTERAKT_TEST_NUMBER is required when INTERAKT_MODE=test")
	}

	log.Printf("✅ Interakt initialized (enabled=%t, mode=%s, template=%s, test_number=%s)", client.runtimeOn, client.runtimeMode, client.runtimeTemplateName, client.testNumber)
	return client, nil
}

func (c *Client) Enabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runtimeOn
}

func (c *Client) UpdateRuntimeSettings(enabled bool, mode string, templateName string, testNumber string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runtimeOn = enabled
	c.runtimeMode = normalizeMode(mode)
	if normalized := normalizeTemplateName(templateName); normalized != "" {
		c.runtimeTemplateName = normalized
	}
	c.testNumber = strings.TrimSpace(testNumber)
	log.Printf("🔁 Interakt runtime settings updated (enabled=%t, mode=%s, template=%s, test_number=%s)", c.runtimeOn, c.runtimeMode, c.runtimeTemplateName, c.testNumber)
}

func (c *Client) SendOrderMessage(ctx context.Context, req SendOrderMessageRequest) (*messageResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("Interakt client is not configured")
	}

	c.mu.RLock()
	enabled := c.runtimeOn
	mode := c.runtimeMode
	testNumber := c.testNumber
	apiKey := c.apiKey
	templateName := c.runtimeTemplateName
	c.mu.RUnlock()

	if !enabled {
		log.Printf("ℹ️  Interakt send skipped for order %s because integration is disabled", strings.TrimSpace(req.OrderID))
		return nil, fmt.Errorf("Interakt integration is disabled")
	}
	if strings.TrimSpace(apiKey) == "" {
		log.Printf("⚠️  Interakt send blocked for order %s because API key is missing", strings.TrimSpace(req.OrderID))
		return nil, fmt.Errorf("INTERAKT_API_KEY is required to send Interakt messages")
	}

	orderID := fallback(req.OrderID, "-")
	customerName := fallback(req.CustomerName, "Customer")
	phone := strings.TrimSpace(req.PhoneNumber)
	originalPhone := phone
	if mode == "test" {
		if strings.TrimSpace(testNumber) == "" {
			log.Printf("⚠️  Interakt send blocked for order %s because test mode is active and test number is empty", orderID)
			return nil, fmt.Errorf("INTERAKT_TEST_NUMBER is required when Interakt is in test mode")
		}
		phone = testNumber
	}
	normalizedPhone, err := normalizeIndianPhone(phone)
	if err != nil {
		log.Printf("⚠️  Interakt send blocked for order %s because phone normalization failed (mode=%s, source_phone=%s, target_phone=%s, err=%v)", orderID, mode, originalPhone, phone, err)
		return nil, err
	}
	log.Printf("📨 Interakt send prepared for order %s (mode=%s, customer=%s, source_phone=%s, target_phone=%s, template=%s)", orderID, mode, customerName, originalPhone, normalizedPhone, normalizeTemplateName(templateName))

	body, err := json.Marshal(messagePayload{
		CountryCode:  defaultCountryCode,
		PhoneNumber:  normalizedPhone,
		CallbackData: strings.TrimSpace(req.CallbackData),
		Type:         "Template",
		Template: templatePayload{
			Name:         normalizeTemplateName(templateName),
			LanguageCode: defaultTemplateLanguageCode,
			BodyValues: []string{
				customerName,
				orderID,
				fallback(req.OrderDetails, "-"),
			},
		},
	})
	if err != nil {
		log.Printf("⚠️  Interakt payload build failed for order %s: %v", orderID, err)
		return nil, fmt.Errorf("marshal Interakt payload: %w", err)
	}

	endpoint := c.baseURL + "/v1/public/message/"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("⚠️  Interakt request creation failed for order %s: %v", orderID, err)
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Basic "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	log.Printf("🚀 Interakt API call started for order %s (mode=%s, phone=%s, endpoint=%s)", orderID, mode, normalizedPhone, endpoint)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("⚠️  Interakt API call failed for order %s (mode=%s, phone=%s): %v", orderID, mode, normalizedPhone, err)
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("⚠️  Interakt response read failed for order %s (mode=%s, phone=%s): %v", orderID, mode, normalizedPhone, err)
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Printf("⚠️  Interakt API returned error for order %s (mode=%s, phone=%s, status=%d, body=%s)", orderID, mode, normalizedPhone, resp.StatusCode, strings.TrimSpace(string(raw)))
		return nil, fmt.Errorf("Interakt send failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decoded messageResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		log.Printf("⚠️  Interakt response decode failed for order %s (mode=%s, phone=%s): %v", orderID, mode, normalizedPhone, err)
		return nil, fmt.Errorf("decode Interakt response: %w", err)
	}
	if !decoded.Result {
		log.Printf("⚠️  Interakt API returned unsuccessful result for order %s (mode=%s, phone=%s, message=%s, id=%s)", orderID, mode, normalizedPhone, decoded.Message, decoded.ID)
		return &decoded, fmt.Errorf("Interakt send failed: %s", decoded.Message)
	}

	log.Printf("✅ Interakt message sent for order %s (mode=%s, phone=%s, message_id=%s, response=%s)", orderID, mode, normalizedPhone, decoded.ID, decoded.Message)
	return &decoded, nil
}
func normalizeIndianPhone(raw string) (string, error) {
	digits := nonDigitPattern.ReplaceAllString(raw, "")
	if len(digits) < 10 {
		return "", fmt.Errorf("phone number must contain at least 10 digits")
	}
	return digits[len(digits)-10:], nil
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "test":
		return "test"
	default:
		return "prod"
	}
}

func normalizeTemplateName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return defaultTemplateName
	}
	return trimmed
}

func fallback(value string, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return strings.TrimSpace(value)
}

package interakt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendOrderMessageBuildsPayloadAndNormalizesPhone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/public/message/" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Basic test-api-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content-type header %q", got)
		}

		expected := `{"countryCode":"+91","phoneNumber":"9876543210","callbackData":"ORDER-1","type":"Template","template":{"name":"amzmrclearorderconfirmation_v2","languageCode":"en","bodyValues":["Ravi","ORDER-1","(Qty:1, 32 x 48 Inch, 1.5mm thick)"]}}`
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(buf) != expected {
			t.Fatalf("unexpected request body:\n%s", string(buf))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":true,"message":"Message created successfully","id":"msg-1"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:      server.URL,
		APIKey:       "test-api-key",
		Enabled:      true,
		Mode:         "prod",
		TemplateName: "amzmrclearorderconfirmation_v2",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.SendOrderMessage(context.Background(), SendOrderMessageRequest{
		CustomerName: "Ravi",
		OrderID:      "ORDER-1",
		OrderDetails: "(Qty:1, 32 x 48 Inch, 1.5mm thick)",
		PhoneNumber:  "+91 98765-43210",
		CallbackData: "ORDER-1",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if resp.ID != "msg-1" {
		t.Fatalf("expected response id msg-1, got %q", resp.ID)
	}
}

func TestSendOrderMessageUsesTestNumberInTestMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		if string(buf) == "" || !strings.Contains(string(buf), `"phoneNumber":"9999988888"`) {
			t.Fatalf("expected test number to be used, got %s", string(buf))
		}
		if !strings.Contains(string(buf), `"type":"Template"`) {
			t.Fatalf("expected template payload, got %s", string(buf))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":true,"message":"ok","id":"msg-2"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:      server.URL,
		APIKey:       "test-api-key",
		Enabled:      true,
		Mode:         "test",
		TestNumber:   "99999 88888",
		TemplateName: "amzmrclearorderconfirmation_v2",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, err := client.SendOrderMessage(context.Background(), SendOrderMessageRequest{
		CustomerName: "Ravi",
		OrderID:      "ORDER-1",
		OrderDetails: "details",
		PhoneNumber:  "1111122222",
	}); err != nil {
		t.Fatalf("send message: %v", err)
	}
}

func TestNormalizeIndianPhoneUsesLastTenDigits(t *testing.T) {
	got, err := normalizeIndianPhone("+91-00000-1234567890")
	if err != nil {
		t.Fatalf("normalize phone: %v", err)
	}
	if got != "1234567890" {
		t.Fatalf("expected last 10 digits, got %q", got)
	}
}

func TestSendOrderMessageUsesRuntimeTemplateName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(buf), `"name":"custom_runtime_template"`) {
			t.Fatalf("expected runtime template name, got %s", string(buf))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":true,"message":"ok","id":"msg-3"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:      server.URL,
		APIKey:       "test-api-key",
		Enabled:      true,
		Mode:         "prod",
		TemplateName: "initial_template",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	client.UpdateRuntimeSettings(true, "prod", "custom_runtime_template", "9999988888")

	if _, err := client.SendOrderMessage(context.Background(), SendOrderMessageRequest{
		CustomerName: "Ravi",
		OrderID:      "ORDER-1",
		OrderDetails: "details",
		PhoneNumber:  "9876543210",
	}); err != nil {
		t.Fatalf("send message: %v", err)
	}
}

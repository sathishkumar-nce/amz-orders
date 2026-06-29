package googlesheets

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Client struct {
	service       *sheets.Service
	spreadsheetID string
}

// NewClient creates a new Google Sheets client
func NewClient(ctx context.Context, credentialsFile, spreadsheetID string) (*Client, error) {
	srv, err := sheets.NewService(ctx, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets client: %w", err)
	}

	return &Client{
		service:       srv,
		spreadsheetID: spreadsheetID,
	}, nil
}

// AppendRow adds a new row to the Google Sheet
func (c *Client) AppendRow(ctx context.Context, values []interface{}) error {
	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{values},
	}

	_, err := c.service.Spreadsheets.Values.Append(
		c.spreadsheetID,
		"Sheet1!A:G", // Range to append to (columns A-G)
		valueRange,
	).ValueInputOption("RAW").Context(ctx).Do()

	if err != nil {
		return fmt.Errorf("unable to append data to sheet: %w", err)
	}

	log.Printf("✅ Successfully added row to Google Sheets")
	return nil
}

// ParsePhoneNumber extracts country code and phone number
func ParsePhoneNumber(phone string) (countryCode string, phoneNumber string) {
	if phone == "" {
		return "", ""
	}

	// Remove all special characters except digits
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")

	// Try to extract country code (assuming 2-3 digit country codes)
	if len(cleaned) >= 10 {
		// Check for India: 91 + 10 digits
		if strings.HasPrefix(cleaned, "91") && len(cleaned) >= 12 {
			return "91", cleaned[2:]
		}
		// Check for US/Canada: 1 + 10 digits
		if strings.HasPrefix(cleaned, "1") && len(cleaned) == 11 {
			return "1", cleaned[1:]
		}
		// Assume 2-digit country code for 12+ digit numbers
		if len(cleaned) >= 12 {
			return cleaned[:2], cleaned[2:]
		}
	}

	// Default: return as-is
	return "", cleaned
}

// FormatAddress formats the delivery address
func FormatAddress(fullname, address, city, state, postcode, country string) string {
	parts := []string{}

	if fullname != "" {
		parts = append(parts, fullname)
	}
	if address != "" {
		parts = append(parts, address)
	}
	if city != "" {
		parts = append(parts, city)
	}
	if state != "" {
		parts = append(parts, state)
	}
	if postcode != "" {
		parts = append(parts, postcode)
	}
	if country != "" {
		parts = append(parts, country)
	}

	return strings.Join(parts, ", ")
}

package googlesheets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

)

const phoneCacheFile = "./phone_cache.json"

// PhoneCache stores phone numbers that already exist in Google Sheets
type PhoneCache struct {
	phones map[string]bool // map[phone_number]exists
	mu     sync.RWMutex
}

// NewPhoneCache creates a new phone cache
func NewPhoneCache() *PhoneCache {
	return &PhoneCache{
		phones: make(map[string]bool),
	}
}

// LoadFromFile loads the phone cache from disk
func (pc *PhoneCache) LoadFromFile() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	file, err := os.Open(phoneCacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, that's okay
			return nil
		}
		return fmt.Errorf("failed to open phone cache file: %w", err)
	}
	defer file.Close()

	var phones []string
	if err := json.NewDecoder(file).Decode(&phones); err != nil {
		return fmt.Errorf("failed to decode phone cache: %w", err)
	}

	// Convert slice to map for fast lookups
	for _, phone := range phones {
		if phone != "" {
			pc.phones[phone] = true
		}
	}

	log.Printf("📞 Loaded %d phone numbers from cache", len(pc.phones))
	return nil
}

// SaveToFile saves the phone cache to disk
func (pc *PhoneCache) SaveToFile() error {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// Convert map to slice
	phones := make([]string, 0, len(pc.phones))
	for phone := range pc.phones {
		phones = append(phones, phone)
	}

	file, err := os.Create(phoneCacheFile)
	if err != nil {
		return fmt.Errorf("failed to create phone cache file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(phones); err != nil {
		return fmt.Errorf("failed to encode phone cache: %w", err)
	}

	return nil
}

// Exists checks if a phone number already exists in the cache
func (pc *PhoneCache) Exists(phone string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if phone == "" {
		return false
	}

	return pc.phones[phone]
}

// Add adds a phone number to the cache
func (pc *PhoneCache) Add(phone string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if phone != "" {
		pc.phones[phone] = true
	}
}

// LoadFromGoogleSheets loads all existing phone numbers from Google Sheets
func (c *Client) LoadPhonesToCache(ctx context.Context) (*PhoneCache, error) {
	cache := NewPhoneCache()

	// Read all phone numbers from column C (Phone Number)
	readRange := "Sheet1!C:C"
	resp, err := c.service.Spreadsheets.Values.Get(c.spreadsheetID, readRange).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data from sheet: %w", err)
	}

	if len(resp.Values) == 0 {
		log.Printf("�� No phone numbers found in Google Sheets")
		return cache, nil
	}

	// Skip header row (index 0) and load all phone numbers
	phoneCount := 0
	for i, row := range resp.Values {
		if i == 0 {
			// Skip header row
			continue
		}

		if len(row) > 0 {
			phone, ok := row[0].(string)
			if ok && phone != "" {
				cache.Add(phone)
				phoneCount++
			}
		}
	}

	log.Printf("📞 Loaded %d phone numbers from Google Sheets", phoneCount)

	// Save to cache file
	if err := cache.SaveToFile(); err != nil {
		log.Printf("⚠️  Warning: Failed to save phone cache to file: %v", err)
	}

	return cache, nil
}


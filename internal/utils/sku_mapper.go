package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
)

type SKUData struct {
	SKU            string
	Thickness      string // NEW: Thickness from CSV (e.g., "1mm", "1.5mm")
	WidthInInches  float64
	LengthInInches float64
	WidthInMM      float64
	LengthInMM     float64
	WeightKg       float64
	IsRound        bool
}

type SKUMapper struct {
	data map[string]*SKUData
	mu   sync.RWMutex
}

func normalizeSKUKey(value string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "\uFEFF"))
	return strings.ToUpper(trimmed)
}

var (
	skuMapperInstance *SKUMapper
	skuMapperOnce     sync.Once
)

// GetSKUMapper returns the singleton instance of SKUMapper
func GetSKUMapper() *SKUMapper {
	skuMapperOnce.Do(func() {
		skuMapperInstance = &SKUMapper{
			data: make(map[string]*SKUData),
		}
	})
	return skuMapperInstance
}

// LoadFromCSV loads SKU data from the CSV file
func (m *SKUMapper) LoadFromCSV(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open SKU CSV file: %w", err)
	}
	defer file.Close()

	data, err := parseSKUCSV(file)
	if err != nil {
		return err
	}

	m.ReplaceData(data)
	return nil
}

func parseSKUCSV(reader io.Reader) (map[string]*SKUData, error) {
	csvReader := csv.NewReader(reader)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read SKU CSV file: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("SKU CSV file is empty or missing data")
	}

	headers := make(map[string]int, len(records[0]))
	for index, header := range records[0] {
		headers[normalizeHeaderKey(header)] = index
	}

	data := make(map[string]*SKUData, len(records)-1)
	for _, record := range records[1:] {
		sku := normalizeSKUKey(getCSVField(record, headers, "sku"))
		if sku == "" {
			continue
		}

		skuData := &SKUData{
			SKU: sku,
		}

		skuData.Thickness = strings.TrimSpace(getCSVField(record, headers, "thickness"))

		if widthInches, err := parseFloat(getCSVField(record, headers, "width in inches")); err == nil {
			skuData.WidthInInches = widthInches
		}
		if lengthInches, err := parseFloat(getCSVField(record, headers, "length in inches")); err == nil {
			skuData.LengthInInches = lengthInches
		}
		if widthMM, err := parseFloat(getCSVField(record, headers, "width in mm")); err == nil {
			skuData.WidthInMM = widthMM
		}
		if lengthMM, err := parseFloat(getCSVField(record, headers, "length in mm")); err == nil {
			skuData.LengthInMM = lengthMM
		}
		if weightKg, err := parseFloat(getCSVField(record, headers, "weight (kg)", "weight")); err == nil {
			skuData.WeightKg = weightKg
		}
		isRoundStr := strings.ToLower(strings.TrimSpace(getCSVField(record, headers, "is_round", "is round")))
		skuData.IsRound = (isRoundStr == "yes" || isRoundStr == "true" || isRoundStr == "1")

		applyDimensionFallback(skuData, getCSVField(record, headers, "dimension"))

		data[sku] = skuData
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("SKU CSV file was parsed but no valid SKU rows were found")
	}

	return data, nil
}

func LoadSKUDataFromCSV(filePath string) (map[string]*SKUData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SKU CSV file: %w", err)
	}
	defer file.Close()

	return parseSKUCSV(file)
}

func (m *SKUMapper) ReplaceData(data map[string]*SKUData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = data
}

// GetBySKU retrieves SKU data by SKU code
func (m *SKUMapper) GetBySKU(sku string) (*SKUData, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sku = normalizeSKUKey(sku)
	data, exists := m.data[sku]
	return data, exists
}

// parseFloat safely parses a float64 from a string
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	return strconv.ParseFloat(s, 64)
}

func normalizeHeaderKey(value string) string {
	replacer := strings.NewReplacer("_", " ", "-", " ")
	normalized := replacer.Replace(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "\uFEFF"))))
	return strings.Join(strings.Fields(normalized), " ")
}

func getCSVField(record []string, headers map[string]int, keys ...string) string {
	for _, key := range keys {
		if index, ok := headers[normalizeHeaderKey(key)]; ok && index < len(record) {
			return strings.TrimSpace(record[index])
		}
	}
	return ""
}

func applyDimensionFallback(skuData *SKUData, dimension string) {
	if skuData.WidthInInches > 0 && skuData.LengthInInches > 0 {
		if skuData.WidthInMM == 0 {
			skuData.WidthInMM = math.Round(skuData.WidthInInches * 25.4)
		}
		if skuData.LengthInMM == 0 {
			skuData.LengthInMM = math.Round(skuData.LengthInInches * 25.4)
		}
		return
	}

	normalized := strings.ToLower(strings.TrimSpace(dimension))
	if normalized == "" {
		return
	}

	if strings.Contains(normalized, "round") {
		if diameter, err := extractFirstNumber(normalized); err == nil {
			skuData.WidthInInches = diameter
			skuData.LengthInInches = diameter
			skuData.WidthInMM = math.Round(diameter * 25.4)
			skuData.LengthInMM = math.Round(diameter * 25.4)
			skuData.IsRound = true
		}
		return
	}

	parts := strings.Split(normalized, "x")
	if len(parts) < 2 {
		return
	}

	width, errWidth := extractFirstNumber(parts[0])
	length, errLength := extractFirstNumber(parts[1])
	if errWidth != nil || errLength != nil {
		return
	}

	skuData.WidthInInches = width
	skuData.LengthInInches = length
	skuData.WidthInMM = math.Round(width * 25.4)
	skuData.LengthInMM = math.Round(length * 25.4)
}

func extractFirstNumber(value string) (float64, error) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	for _, field := range fields {
		if field == "" {
			continue
		}
		return strconv.ParseFloat(field, 64)
	}
	return 0, fmt.Errorf("no numeric value found")
}

// GetDataCount returns the number of SKUs loaded
func (m *SKUMapper) GetDataCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestDirectOrderPaginationNormalizesBeforeDTDCResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		query       string
		page, limit int
	}{
		{"?page=0&limit=0", 1, 20}, {"?page=-4&limit=-2", 1, 20}, {"?page=abc&limit=abc", 1, 20}, {"?page=2&limit=9999", 2, 500}, {"?page=3&limit=25", 3, 25},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/"+tt.query, nil)
		f := parseDirectOrderFilters(c)
		if f.Page != tt.page || f.Limit != tt.limit {
			t.Fatalf("%s: page=%d limit=%d", tt.query, f.Page, f.Limit)
		}
	}
}

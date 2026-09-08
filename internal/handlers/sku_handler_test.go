package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUpdateSKUFileDoesNotChangeMapperDirectoryPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("set mapper directory permissions: %v", err)
	}

	mapperPath := filepath.Join(dir, "SKU_V8.csv")
	initialCSV := "SKU,Weight (kg)\nOLD-SKU,1\n"
	if err := os.WriteFile(mapperPath, []byte(initialCSV), 0o600); err != nil {
		t.Fatalf("write initial mapper: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "SKU_V9.csv")
	if err != nil {
		t.Fatalf("create upload form field: %v", err)
	}
	if _, err := part.Write([]byte("SKU,Weight (kg)\nNEW-SKU,2.5\n")); err != nil {
		t.Fatalf("write upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sku-mapper", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	handler := NewSKUHandler(mapperPath)
	handler.UpdateSKUFile(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat mapper directory: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("mapper directory permissions changed: got %o, want 700", got)
	}

	updatedCSV, err := os.ReadFile(mapperPath)
	if err != nil {
		t.Fatalf("read updated mapper: %v", err)
	}
	if !bytes.Contains(updatedCSV, []byte("NEW-SKU,2.5")) {
		t.Fatalf("mapper was not replaced: %q", updatedCSV)
	}
}

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/delhivery"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/dtdc"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
)

func dtdcTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DTDC_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set DTDC_TEST_DATABASE_URL to a dedicated localhost codex_dtdc_test database")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	if (cfg.ConnConfig.Host != "127.0.0.1" && cfg.ConnConfig.Host != "localhost") || cfg.ConnConfig.Database != "codex_dtdc_test" {
		t.Fatal("integration tests require a dedicated localhost codex_dtdc_test database")
	}
	ctx := context.Background()
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("dtdc_it_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg = cfg.Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		if err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "db-migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatal("migration files not found")
	}
	sort.Strings(files)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.Split(string(data), "-- +goose Down")[0]
		if _, err = pool.Exec(ctx, up); err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(path), err)
		}
	}
	return pool
}

func TestDTDCPersistedLifecycle(t *testing.T) {
	pool := dtdcTestDatabase(t)
	gin.SetMode(gin.TestMode)
	var mu sync.Mutex
	modes := map[string]string{}
	calls := map[string]int{}
	awbs := map[string]string{}
	cancelled := map[string]bool{}
	legacyRequests := []map[string]any{}
	carrier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/softdata") {
			var payload struct {
				Consignments []map[string]any `json:"consignments"`
			}
			if json.NewDecoder(r.Body).Decode(&payload) != nil || len(payload.Consignments) != 1 {
				http.Error(w, "bad payload", 400)
				return
			}
			id, _ := payload.Consignments[0]["customer_reference_number"].(string)
			calls[id]++
			if r.Header.Get("api-key") != "dtdc-test-key" {
				t.Error("incorrect DTDC credentials")
			}
			if payload.Consignments[0]["weight"] != "0.5" {
				t.Error("grams not converted to kg")
			}
			if modes[id] == "rejected" {
				json.NewEncoder(w).Encode(map[string]any{"status": "OK", "data": []any{map[string]any{"success": false, "message": "NO_SERIES_AVAILABLE"}}})
				return
			}
			awbs[id] = "TEST-" + id
			if modes[id] == "ambiguous" {
				fmt.Fprint(w, "truncated response")
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"status": "OK", "data": []any{map[string]any{"success": true, "reference_number": awbs[id]}}})
			return
		}
		if strings.HasSuffix(r.URL.Path, "getTrackDetails") {
			var payload map[string]string
			json.NewDecoder(r.Body).Decode(&payload)
			id := payload["strcnno"]
			if payload["trkType"] == "cnno" {
				id = strings.TrimPrefix(id, "TEST-")
			}
			if r.Header.Get("x-access-token") != "dtdc-test-track" {
				t.Error("incorrect tracking credentials")
			}
			if modes[id] == "no-data" {
				json.NewEncoder(w).Encode(map[string]any{"status": "FAILED", "statusFlag": false})
				return
			}
			reference := id
			if modes[id] == "wrong-reference" {
				reference = "ANOTHER-ORDER"
			}
			json.NewEncoder(w).Encode(map[string]any{"statusFlag": true, "status": "SUCCESS", "trackHeader": map[string]string{"strShipmentNo": awbs[id], "strRefNo": reference, "strCNActCustCode": "TEST-ACCOUNT", "strStatus": "In Transit"}, "trackDetails": []map[string]string{{"strAction": "Booked", "strActionDate": "05112026", "strActionTime": "1130"}}})
			return
		}
		if strings.Contains(r.URL.Path, "shippinglabel") {
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, "%PDF-integration-test")
			return
		}
		if strings.HasSuffix(r.URL.Path, "/cancel") {
			var payload struct {
				AWBs     []string `json:"AWBNo"`
				Customer string   `json:"customerCode"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			if len(payload.AWBs) != 1 || payload.Customer != "TEST-ACCOUNT" {
				http.Error(w, "bad cancellation", 400)
				return
			}
			cancelled[payload.AWBs[0]] = true
			json.NewEncoder(w).Encode(map[string]any{"status": "OK", "success": true, "successConsignments": []any{map[string]any{"success": true, "reference_number": payload.AWBs[0]}}})
			return
		}

		if r.URL.Path == "/api/v1/packages/json/" {
			awb := r.URL.Query().Get("waybill")
			if r.Header.Get("Authorization") != "Token delhivery-test-key" {
				t.Error("wrong Delhivery tracking credentials")
			}
			json.NewEncoder(w).Encode(map[string]any{"ShipmentData": []any{map[string]any{"Shipment": map[string]any{"AWB": awb, "Origin": "Namakkal", "Destination": "Chennai", "ExpectedDeliveryDate": "2026-11-07", "Status": map[string]any{"Status": "In Transit", "StatusLocation": "Chennai Hub", "StatusDateTime": "2026-11-05T11:30:00", "Instructions": "On the way"}, "Scans": []any{map[string]any{"ScanDetail": map[string]any{"Scan": "Received at hub", "ScannedLocation": "Chennai Hub", "ScanDateTime": "2026-11-05T11:30:00"}}}}}}})
			return
		}
		if r.URL.Path == "/api/cmu/create.json" {
			r.ParseForm()
			var payload struct {
				Shipments []map[string]any  `json:"shipments"`
				Pickup    map[string]string `json:"pickup_location"`
			}
			json.Unmarshal([]byte(r.PostForm.Get("data")), &payload)
			if len(payload.Shipments) != 1 {
				http.Error(w, "bad legacy payload", 400)
				return
			}
			shipment := payload.Shipments[0]
			id := shipment["order"].(string)
			calls[id]++
			legacyRequests = append(legacyRequests, shipment)
			if payload.Pickup["name"] != "Legacy pickup" || r.Header.Get("Authorization") != "Token delhivery-test-key" {
				t.Error("legacy Delhivery pickup/credentials changed")
			}
			json.NewEncoder(w).Encode(map[string]any{"success": true, "packages": []any{map[string]any{"status": "Success", "waybill": "LEGACY-" + id, "refnum": id}}})
			return
		}
		http.NotFound(w, r)
	}))
	defer carrier.Close()
	repo := repository.NewDirectOrderRepository(pool)
	legacy := delhivery.NewClient(delhivery.Config{BaseURL: carrier.URL, APIToken: "delhivery-test-key", SellerGSTIN: "legacy-gstin", SellerName: "Legacy seller", SellerAddress: "Legacy seller address", DefaultPickupLocation: "Legacy pickup"})
	handler := NewDirectOrderHandler(repo, legacy)
	client := dtdc.NewClient(dtdc.Config{BaseURL: carrier.URL, TrackingURL: carrier.URL, APIKey: "dtdc-test-key", TrackingToken: "dtdc-test-track", CustomerCode: "TEST-ACCOUNT", ServiceType: "B2C SMART EXPRESS", Commodity: "84", Origin: dtdc.Address{Name: "Seller", Phone: "9999999999", Address: "Seller street", Pincode: "637001", City: "Namakkal", State: "Tamil Nadu"}})
	handler.SetDTDCClient(client)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("username", "integration-test") })
	group := router.Group("/api/v1/direct-orders")
	group.POST("", handler.CreateDirectOrder)
	group.GET("/courier-settings", handler.GetCourierSettings)
	group.PUT("/courier-settings", handler.UpdateCourierSettings)
	group.GET("/dtdc/settings", handler.DTDCSettings)
	group.GET("/shipments", handler.ListCourierShipments)
	group.GET("/search", handler.SearchDirectOrders)
	group.POST("/:order_id/tracking", handler.TrackShipment)
	group.GET("/dtdc/shipments", handler.ListDTDCShipments)
	group.POST("/:order_id/dtdc/book", handler.BookDTDC)
	group.POST("/:order_id/dtdc/track", handler.TrackDTDC)
	group.POST("/:order_id/dtdc/reconcile", handler.ReconcileDTDC)
	group.POST("/:order_id/dtdc/cancel", handler.CancelDTDC)
	group.GET("/:order_id/dtdc/label", handler.LabelDTDC)
	group.PATCH("/:order_id", handler.UpdateDirectOrder)
	group.DELETE("/:order_id", handler.DeleteDirectOrder)
	group.POST("/:order_id/delhivery/forward-order", handler.CreateDelhiveryForwardOrder)
	perform := func(method, path string, body any) *httptest.ResponseRecorder {
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		request := httptest.NewRequest(method, "/api/v1/direct-orders"+path, bytes.NewReader(data))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	expect := func(response *httptest.ResponseRecorder, status int) {
		t.Helper()
		if response.Code != status {
			t.Fatalf("HTTP %d, expected %d: %s", response.Code, status, response.Body.String())
		}
	}
	create := func(id string) {
		t.Helper()
		expect(perform("POST", "", map[string]any{"source": "website", "order_id": id, "customer_name": "Customer", "address": "Customer street", "mobile": "8888888888", "pincode": "600001", "city": "Chennai", "state": "Tamil Nadu", "payment_status": "paid-full", "amount": 100, "cod_amount": 0, "total_weight": 500, "length_cm": 10, "width_cm": 10, "height_cm": 5, "pickup_location": "Legacy pickup", "items": []map[string]any{{"item": "Product", "quantity": 1, "hsn": "3926", "weight": 500}}}), 201)
	}
	get := func(id string) *models.DirectOrder {
		t.Helper()
		order, err := repo.GetByOrderID(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		return order
	}
	setMode := func(id, mode string) { mu.Lock(); modes[id] = mode; mu.Unlock() }
	count := func(id string) int { mu.Lock(); defer mu.Unlock(); return calls[id] }
	t.Run("book track label cancel persist", func(t *testing.T) {
		id := "IT-LIFECYCLE"
		create(id)
		expect(perform("POST", "/"+id+"/dtdc/book", map[string]any{}), 200)
		order := get(id)
		if order.AWB.String != "TEST-"+id || order.OrderStatus != "forwarded" || order.ServiceType.String != "B2C SMART EXPRESS" {
			t.Fatalf("booking not persisted: %+v", order)
		}
		var payload map[string]any
		json.Unmarshal(order.CourierPayload, &payload)
		if payload["booking_request"] == nil || payload["booking"] == nil {
			t.Fatal("booking request/response audit missing")
		}
		response := perform("POST", "/"+id+"/dtdc/book", map[string]any{})
		if response.Code != 400 && response.Code != 409 {
			t.Fatal("duplicate not blocked")
		}
		if count(id) != 1 {
			t.Fatal("duplicate reached carrier")
		}
		expect(perform("POST", "/"+id+"/delhivery/forward-order", nil), 409)
		if count(id) != 1 {
			t.Fatal("cross-courier duplicate reached carrier")
		}
		expect(perform("PATCH", "/"+id, map[string]any{"awb": "WRONG"}), 409)
		expect(perform("POST", "/"+id+"/dtdc/track", nil), 200)
		order = get(id)
		if order.CourierStatus.String != "In Transit" {
			t.Fatal("tracking status not persisted")
		}
		response = perform("GET", "/"+id+"/dtdc/label", nil)
		expect(response, 200)
		if !bytes.HasPrefix(response.Body.Bytes(), []byte("%PDF-")) {
			t.Fatal("label missing")
		}
		expect(perform("POST", "/"+id+"/dtdc/cancel", nil), 200)
		expect(perform("POST", "/"+id+"/dtdc/cancel", nil), 200)
		if get(id).CourierStatus.String != "Cancelled" {
			t.Fatal("cancellation not persisted")
		}
		expect(perform("POST", "/"+id+"/dtdc/track", nil), 200)
		if get(id).CourierStatus.String != "Cancelled" {
			t.Fatal("cancelled shipment scan refresh changed its status")
		}
		if err := repo.SaveDTDCEvent(context.Background(), id, "TEST-"+id, "In Transit", "tracking", json.RawMessage(`{}`), "late-tracking"); err == nil {
			t.Fatal("late tracking overwrote cancellation")
		}
		expect(perform("DELETE", "/"+id, nil), 409)
		response = perform("GET", "/dtdc/shipments?search="+id+"&courier_status=Cancelled&page=0&limit=0", nil)
		expect(response, 200)
		var list struct {
			Total, Page, Limit int
			Data               []models.DirectOrderResponse
		}
		if json.Unmarshal(response.Body.Bytes(), &list) != nil || list.Total != 1 || list.Page != 1 || list.Limit != 20 || len(list.Data) != 1 {
			t.Fatalf("shipment list incorrect: %s", response.Body.String())
		}
	})
	t.Run("definite rejection can be retried", func(t *testing.T) {
		id := "IT-REJECTION"
		create(id)
		setMode(id, "rejected")
		expect(perform("POST", "/"+id+"/dtdc/book", map[string]any{}), 502)
		order := get(id)
		if order.CourierStatus.String != "booking_rejected" || !bytes.Contains(order.CourierPayload, []byte("NO_SERIES_AVAILABLE")) {
			t.Fatal("rejection not retained")
		}
		setMode(id, "")
		expect(perform("POST", "/"+id+"/dtdc/book", map[string]any{}), 200)
		if count(id) != 2 || bytes.Contains(get(id).CourierPayload, []byte("booking_error")) {
			t.Fatal("retry result or error cleanup incorrect")
		}
	})
	t.Run("ambiguous booking recovered without rebooking", func(t *testing.T) {
		id := "IT-RECOVERY"
		create(id)
		setMode(id, "ambiguous")
		expect(perform("POST", "/"+id+"/dtdc/book", map[string]any{}), 502)
		if get(id).CourierStatus.String != "booking_review" {
			t.Fatal("ambiguous booking not locked")
		}
		expect(perform("POST", "/"+id+"/dtdc/book", map[string]any{}), 409)
		setMode(id, "no-data")
		expect(perform("POST", "/"+id+"/dtdc/reconcile", nil), 502)
		if get(id).AWB.String != "" {
			t.Fatal("no-data unlocked booking")
		}
		setMode(id, "wrong-reference")
		expect(perform("POST", "/"+id+"/dtdc/reconcile", nil), 502)
		if get(id).AWB.String != "" {
			t.Fatal("wrong reference linked")
		}
		setMode(id, "")
		expect(perform("POST", "/"+id+"/dtdc/reconcile", nil), 200)
		if get(id).AWB.String != "TEST-"+id || count(id) != 1 {
			t.Fatal("recovery did not reuse existing shipment")
		}
		expect(perform("POST", "/"+id+"/dtdc/reconcile", nil), 409)
	})
	t.Run("fresh pending cannot be reconciled", func(t *testing.T) {
		id := "IT-PENDING"
		create(id)
		order := get(id)
		payload, _ := client.BuildBooking(order, dtdc.BookingOptions{})
		raw, _ := json.Marshal(payload)
		if err := repo.ClaimDTDCBooking(context.Background(), id, "test", order.UpdatedAt, raw); err != nil {
			t.Fatal(err)
		}
		expect(perform("POST", "/"+id+"/dtdc/reconcile", nil), 409)
	})
	t.Run("concurrent couriers only create one shipment", func(t *testing.T) {
		id := "IT-RACE"
		create(id)
		responses := make(chan *httptest.ResponseRecorder, 2)
		var wg sync.WaitGroup
		for _, path := range []string{"/" + id + "/dtdc/book", "/" + id + "/delhivery/forward-order"} {
			wg.Add(1)
			go func(path string) { defer wg.Done(); responses <- perform("POST", path, map[string]any{}) }(path)
		}
		wg.Wait()
		close(responses)
		success := 0
		for response := range responses {
			if response.Code == 200 {
				success++
			} else if response.Code != 409 && response.Code != 400 {
				t.Fatalf("unexpected concurrent result %d: %s", response.Code, response.Body.String())
			}
		}
		if success != 1 || count(id) != 1 {
			t.Fatalf("concurrent booking count: success=%d carrier calls=%d", success, count(id))
		}
	})
	t.Run("legacy Delhivery persists original mapping", func(t *testing.T) {
		id := "IT-LEGACY"
		create(id)
		expect(perform("POST", "/"+id+"/delhivery/forward-order", nil), 200)
		order := get(id)
		if order.CourierType.String != "delhivery" || order.AWB.String != "LEGACY-"+id || order.ServiceType.String != "surface" {
			t.Fatal("legacy persistence changed")
		}
		mu.Lock()
		defer mu.Unlock()
		last := legacyRequests[len(legacyRequests)-1]
		if last["seller_add"] != "Legacy seller address" || last["weight"] != float64(500) || last["payment_mode"] != "Pre-paid" {
			t.Fatalf("legacy payload changed: %v", last)
		}
	})
	t.Run("runtime modes persist and affect booking without restart", func(t *testing.T) {
		response := perform("GET", "/courier-settings", nil)
		expect(response, 200)
		var settings models.CourierSettings
		if err := json.Unmarshal(response.Body.Bytes(), &settings); err != nil {
			t.Fatal(err)
		}
		settings.DTDCModes = append(settings.DTDCModes, models.CourierServiceMode{Code: "b2b runtime", Label: "Runtime B2B", Enabled: true})
		settings.DTDCDefaultService = "B2B RUNTIME"
		settings.AutoDownloadDTDCLabels = false
		settings.DTDCLabelCode = "SHIP_LABEL_A4"
		response = perform("PUT", "/courier-settings", settings)
		expect(response, 200)
		if err := json.Unmarshal(response.Body.Bytes(), &settings); err != nil {
			t.Fatal(err)
		}
		response = perform("GET", "/dtdc/settings", nil)
		expect(response, 200)
		var metadata struct {
			Service string   `json:"service_type"`
			Options []string `json:"service_type_options"`
			Auto    bool     `json:"auto_download_labels"`
			Label   string   `json:"label_code"`
		}
		json.Unmarshal(response.Body.Bytes(), &metadata)
		if metadata.Service != "B2B RUNTIME" || metadata.Auto || metadata.Label != "SHIP_LABEL_A4" {
			t.Fatalf("runtime defaults not applied: %s", response.Body.String())
		}
		id := "IT-RUNTIME"
		create(id)
		expect(perform("POST", "/"+id+"/dtdc/book", map[string]any{}), 200)
		if get(id).ServiceType.String != "B2B RUNTIME" {
			t.Fatal("booking ignored runtime service default")
		}
		for i := range settings.DTDCModes {
			if settings.DTDCModes[i].Code == "B2B RUNTIME" {
				settings.DTDCModes[i].Enabled = false
			}
		}
		settings.DTDCDefaultService = "B2C SMART EXPRESS"
		expect(perform("PUT", "/courier-settings", settings), 200)
		id = "IT-DISABLED"
		create(id)
		expect(perform("POST", "/"+id+"/dtdc/book", map[string]any{"service_type_id": "B2B RUNTIME"}), 400)
		if count(id) != 0 {
			t.Fatal("disabled service reached carrier")
		}
		settings.DTDCModes = append(settings.DTDCModes, models.CourierServiceMode{Code: "b2c smart express", Enabled: true})
		expect(perform("PUT", "/courier-settings", settings), 400)
	})
	t.Run("both couriers appear in a shared shipment list", func(t *testing.T) {
		response := perform("GET", "/shipments?courier=delhivery&search=IT-LEGACY", nil)
		expect(response, 200)
		var list struct {
			Total int                          `json:"total"`
			Data  []models.DirectOrderResponse `json:"data"`
		}
		json.Unmarshal(response.Body.Bytes(), &list)
		if list.Total != 1 || len(list.Data) != 1 {
			t.Fatalf("Delhivery shipment missing: %s", response.Body.String())
		}
		response = perform("GET", "/shipments?courier=dtdc&search=IT-LEGACY", nil)
		expect(response, 200)
		json.Unmarshal(response.Body.Bytes(), &list)
		if list.Total != 0 {
			t.Fatal("courier filter ignored")
		}
		response = perform("GET", "/search?search=%2B91%2088888%2088888", nil)
		expect(response, 200)
		json.Unmarshal(response.Body.Bytes(), &list)
		if list.Total < 2 {
			t.Fatal("normalized phone search did not find direct orders")
		}
	})
	t.Run("official tracking summaries are persisted for each courier", func(t *testing.T) {
		response := perform("POST", "/IT-LEGACY/tracking", nil)
		expect(response, 200)
		var result struct {
			Tracking models.ShipmentTracking `json:"tracking"`
		}
		if json.Unmarshal(response.Body.Bytes(), &result) != nil {
			t.Fatal("invalid tracking response")
		}
		if result.Tracking.Courier != "delhivery" || result.Tracking.CurrentLocation != "Chennai Hub" || len(result.Tracking.Events) != 1 {
			t.Fatalf("Delhivery tracking not normalized: %s", response.Body.String())
		}
		if !bytes.Contains(get("IT-LEGACY").CourierPayload, []byte("tracking_summary")) {
			t.Fatal("shared tracking summary not saved")
		}
		response = perform("POST", "/IT-LIFECYCLE/tracking", nil)
		expect(response, 200)
		json.Unmarshal(response.Body.Bytes(), &result)
		if result.Tracking.Status != "Cancelled" || result.Tracking.ReportedStatus != "In Transit" || get("IT-LIFECYCLE").CourierStatus.String != "Cancelled" {
			t.Fatal("tracking summary overwrote cancellation")
		}
		expect(perform("POST", "/IT-DISABLED/tracking", nil), 400)
	})

}

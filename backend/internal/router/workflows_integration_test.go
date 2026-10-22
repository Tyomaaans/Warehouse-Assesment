package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"warehouse/internal/auth/session"
	"warehouse/internal/auth/token"
	"warehouse/internal/auth/user"
	"warehouse/internal/infrastructure/postgres"
	redisinfra "warehouse/internal/infrastructure/redis"
	"warehouse/internal/inventory"
	"warehouse/internal/middleware"
	"warehouse/internal/notifications"
	"warehouse/internal/products"
	"warehouse/internal/receiving"
	"warehouse/internal/receiving/domain"
	"warehouse/internal/router"
	"warehouse/migrations"
	"warehouse/pkg"
)

type responseEvidence struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

type workflowFixture struct {
	db       *gorm.DB
	router   *gin.Engine
	users    map[string]user.UserStorage
	tokens   map[string]string
	mu       sync.Mutex
	evidence []responseEvidence
}

func newWorkflowFixture(t *testing.T) *workflowFixture {
	t.Helper()
	databaseURL, redisURL := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_REDIS_URL")
	if databaseURL == "" || redisURL == "" {
		t.Skip("set TEST_DATABASE_URL and TEST_REDIS_URL for PostgreSQL/Redis integration")
	}
	ctx := context.Background()
	admin, err := postgres.NewPostgresDB(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminPool, _ := admin.DB()
	t.Cleanup(func() { _ = adminPool.Close() })
	schema := "documents_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error })
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal("invalid test database URL")
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := postgres.NewPostgresDB(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	t.Cleanup(func() { _ = pool.Close() })
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal("migrations:", err)
	}
	if err := migrations.Check(ctx, db); err != nil {
		t.Fatal("migration checksums:", err)
	}
	rc, err := redisinfra.NewRedisClient(ctx, redisURL, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	f := &workflowFixture{db: db, users: make(map[string]user.UserStorage), tokens: make(map[string]string)}
	password := "task2c-test-password"
	accounts := []user.ProvisionAccount{
		{Email: uuid.NewString() + "@example.test", Password: password, Role: pkg.Operator},
		{Email: uuid.NewString() + "@example.test", Password: password, Role: pkg.Supervisor},
	}
	if count, err := user.ProvisionUsers(ctx, db, pkg.NewValidator(), accounts); err != nil || count != 2 {
		t.Fatalf("provisioned count = %d, error = %v", count, err)
	}
	userRepo := user.NewUserRepository(db)
	tokenSvc := token.NewJWTService("task2c-integration-signing-secret-at-least-32-bytes", 168*time.Hour, time.Hour, rc, session.NewSessionService(rc), userRepo)
	authMiddleware := middleware.NewAuthMiddleware(tokenSvc)
	authHandler := user.NewUserHandler(user.NewUserService(userRepo, tokenSvc, pkg.NewValidator()), false)
	productHandler := products.NewProductHandler(products.NewProductService(products.NewProductRepository(db), pkg.NewValidator()))
	receivingHandler := receiving.NewReceivingHandler(receiving.NewReceivingService(receiving.NewReceivingRepository(db), pkg.NewValidator()))
	gin.SetMode(gin.ReleaseMode)
	inventoryHandler := inventory.NewInventoryHandler(inventory.NewInventoryService(inventory.NewInventoryRepository(db)))
	notificationHandler := notifications.NewNotificationHandler(notifications.NewNotificationService(notifications.NewNotificationRepository(db)))
	f.router, err = router.NewRouter(zerolog.Nop(), func(context.Context) error { return nil }, nil, authHandler, authMiddleware, productHandler, receivingHandler, inventoryHandler, notificationHandler)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range accounts {
		var stored user.UserStorage
		if err := db.Where("email = ?", account.Email).First(&stored).Error; err != nil {
			t.Fatal(err)
		}
		f.users[account.Role] = stored
		response := f.request("POST", "/api/v1/auth/login", user.LoginRequest{Email: account.Email, Password: password}, "")
		if response.Code != 200 {
			t.Fatalf("login status = %d", response.Code)
		}
		data := responseData[user.LoginResponse](t, response)
		f.tokens[account.Role] = data.AccessToken
		var sessionID uuid.UUID
		var refresh string
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == "session_id" {
				sessionID, err = pkg.ParseID(cookie.Value)
				if err != nil {
					t.Fatal(err)
				}
			}
			if cookie.Name == "refresh_token" {
				refresh = cookie.Value
			}
		}
		t.Cleanup(func() {
			_ = rc.Del(ctx, "session:"+session.HashID(sessionID.String()), "refresh:"+session.HashID(refresh), "user_sessions:"+stored.ID.String()).Err()
		})
	}
	if path := os.Getenv("CONTRACT_EVIDENCE_PATH"); path != "" {
		path = strings.ReplaceAll(path, "{test}", t.Name())
		t.Cleanup(func() {
			data, err := json.MarshalIndent(f.evidence, "", "  ")
			if err != nil {
				t.Error(err)
				return
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Error(err)
			}
		})
	}
	return f
}

func (f *workflowFixture) request(method, path string, payload any, role string) *httptest.ResponseRecorder {
	var body []byte
	if raw, ok := payload.(string); ok {
		body = []byte(raw)
	} else if payload != nil {
		body, _ = json.Marshal(payload)
	}
	req := httptest.NewRequest(method, "http://warehouse.test"+path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	if access := f.tokens[role]; access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, req)
	operation := ""
	parts := strings.Split(strings.Split(path, "?")[0], "/")
	for _, route := range f.router.Routes() {
		template := strings.Split(route.Path, "/")
		if route.Method != method || len(template) != len(parts) {
			continue
		}
		matches := true
		for i := range parts {
			if template[i] != parts[i] && template[i] != ":id" {
				matches = false
			}
		}
		if matches {
			operation = strings.ReplaceAll(route.Path, ":id", "{id}")
			break
		}
	}
	if operation != "" && !strings.HasPrefix(operation, "/api/v1/auth/") {
		f.mu.Lock()
		f.evidence = append(f.evidence, responseEvidence{Method: method, Path: strings.TrimPrefix(operation, "/api/v1"), Status: response.Code, Body: append(json.RawMessage(nil), response.Body.Bytes()...)})
		f.mu.Unlock()
	}
	return response
}

func responseData[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Success bool `json:"success"`
		Data    T    `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || !envelope.Success {
		t.Fatalf("invalid success envelope; status = %d", response.Code)
	}
	return envelope.Data
}

func workflowError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var envelope map[string]json.RawMessage
	var detail map[string]string
	if response.Code != status || json.Unmarshal(response.Body.Bytes(), &envelope) != nil || len(envelope) != 2 || string(envelope["success"]) != "false" || json.Unmarshal(envelope["error"], &detail) != nil || len(detail) != 2 || detail["code"] != code || detail["message"] == "" {
		t.Fatalf("status/code = %d/%s, want %d/%s", response.Code, detail["code"], status, code)
	}
	for _, sensitive := range []string{"SQLSTATE", "violates", "_fkey", "_key", "password_hash"} {
		if strings.Contains(detail["message"], sensitive) {
			t.Fatal("error exposed database or credential details")
		}
	}
}

func tableCount(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func TestProductReceivingIntegration(t *testing.T) {
	f := newWorkflowFixture(t)
	var productA, productB products.ProductResponse
	var document receiving.ReceivingResponse
	t.Run("empty_pages_and_read_authentication", func(t *testing.T) {
		for _, role := range []string{pkg.Operator, pkg.Supervisor} {
			productsPage := responseData[products.PaginatedProductResponse](t, f.request("GET", "/api/v1/products", nil, role))
			receivingsPage := responseData[receiving.PaginatedReceivingResponse](t, f.request("GET", "/api/v1/receivings", nil, role))
			if productsPage.Products == nil || receivingsPage.Receivings == nil || productsPage.Meta != (pkg.PaginationMeta{CurrentPage: 1, PageSize: 10}) || receivingsPage.Meta != productsPage.Meta {
				t.Fatal("incorrect empty collection or default pagination")
			}
		}
		for _, path := range []string{"/api/v1/products", "/api/v1/receivings", "/api/v1/receivings/" + pkg.EncodeID(uuid.New())} {
			workflowError(t, f.request("GET", path, nil, ""), 401, "unauthorized")
		}
	})
	t.Run("product_creation_permissions_validation_and_duplicate", func(t *testing.T) {
		valid := products.CreateProductRequest{SKU: "SKU-A", Name: "Product A"}
		workflowError(t, f.request("POST", "/api/v1/products", valid, ""), 401, "unauthorized")
		workflowError(t, f.request("POST", "/api/v1/products", valid, pkg.Operator), 403, "forbidden")
		if tableCount(t, f.db, "products") != 0 {
			t.Fatal("denied product creation wrote data")
		}
		for _, body := range []string{`null`, `{"sku":"","name":"A"}`, `{"sku":" \t","name":"A"}`, `{"sku":"A"}`, `{"sku":"A","name":" "}`, `{"sku":"A","name":"A","role":"SUPERVISOR"}`} {
			workflowError(t, f.request("POST", "/api/v1/products", body, pkg.Supervisor), 400, "invalid_input")
		}
		response := f.request("POST", "/api/v1/products", valid, pkg.Supervisor)
		if response.Code != 201 {
			t.Fatalf("product create status = %d", response.Code)
		}
		productA = responseData[products.ProductResponse](t, response)
		if _, err := pkg.ParseID(productA.ID); err != nil || productA.SKU != valid.SKU || productA.CreatedAt.IsZero() {
			t.Fatal("invalid product ID/fields")
		}
		workflowError(t, f.request("POST", "/api/v1/products", valid, pkg.Supervisor), 409, "already_exists")
		productB = responseData[products.ProductResponse](t, f.request("POST", "/api/v1/products", products.CreateProductRequest{SKU: "SKU-B", Name: "Product B"}, pkg.Supervisor))
		for _, table := range []string{"inventory_balances", "inventory_transactions"} {
			if tableCount(t, f.db, table) != 0 {
				t.Fatal("product creation wrote inventory")
			}
		}
	})
	t.Run("product_search_and_pagination", func(t *testing.T) {
		special := products.CreateProductRequest{SKU: `SKU-%_\`, Name: "Literal symbols"}
		response := f.request("POST", "/api/v1/products", special, pkg.Supervisor)
		if response.Code != 201 {
			t.Fatalf("special SKU create status = %d", response.Code)
		}
		for _, search := range []string{"product a", "%", "_", `\`} {
			page := responseData[products.PaginatedProductResponse](t, f.request("GET", "/api/v1/products?search="+url.QueryEscape(search), nil, pkg.Operator))
			if len(page.Products) != 1 || page.Meta.TotalData != 1 {
				t.Fatalf("literal/case-insensitive search %q matched %d products", search, len(page.Products))
			}
		}
		page1 := responseData[products.PaginatedProductResponse](t, f.request("GET", "/api/v1/products?limit=1", nil, pkg.Operator))
		page2 := responseData[products.PaginatedProductResponse](t, f.request("GET", "/api/v1/products?limit=1&page=2", nil, pkg.Supervisor))
		if len(page1.Products) != 1 || len(page2.Products) != 1 || page1.Products[0].ID == page2.Products[0].ID || page1.Meta.TotalData != 3 || page1.Meta.TotalPages != 3 {
			t.Fatal("incorrect product pagination")
		}
		outside := responseData[products.PaginatedProductResponse](t, f.request("GET", "/api/v1/products?page=10000", nil, pkg.Operator))
		if outside.Products == nil || len(outside.Products) != 0 || outside.Meta.TotalData != 3 {
			t.Fatal("out-of-range product page lost totals or empty array")
		}
		for _, query := range []string{"page=0", "page=10001", "page=abc", "limit=0", "limit=101", "search=" + strings.Repeat("x", 101)} {
			workflowError(t, f.request("GET", "/api/v1/products?"+query, nil, pkg.Operator), 400, "invalid_input")
		}
	})
	t.Run("receiving_creation_permissions_snapshots_and_audit", func(t *testing.T) {
		payload := receiving.CreateReceivingRequest{OrderReference: "ORD-001", Items: []receiving.ExpectedItemRequest{
			{ProductID: productA.ID, ExpectedQuantity: 130}, {ProductID: productB.ID, ExpectedQuantity: 60},
		}}
		workflowError(t, f.request("POST", "/api/v1/receivings", payload, ""), 401, "unauthorized")
		workflowError(t, f.request("POST", "/api/v1/receivings", payload, pkg.Supervisor), 403, "forbidden")
		if tableCount(t, f.db, "receiving_documents") != 0 {
			t.Fatal("denied receiving creation wrote data")
		}
		response := f.request("POST", "/api/v1/receivings", payload, pkg.Operator)
		if response.Code != 201 {
			t.Fatalf("receiving create status = %d", response.Code)
		}
		document = responseData[receiving.ReceivingResponse](t, response)
		if document.Status != "INCOMPLETE" || document.CompletedAt != nil || document.CreatedBy != pkg.EncodeID(f.users[pkg.Operator].ID) || len(document.Items) != 2 || document.CreatedAt.IsZero() || !document.CreatedAt.Equal(document.UpdatedAt) {
			t.Fatal("incorrect initial document state or trusted actor/time")
		}
		quantities := map[string]int64{productA.ID: 130, productB.ID: 60}
		for _, item := range document.Items {
			if _, err := pkg.ParseID(item.ID); err != nil || item.ExpectedQuantity != quantities[item.ProductID] || item.ApprovedQuantity != 0 || item.RemainingQuantity != item.ExpectedQuantity || item.SKU == "" || item.Name == "" {
				t.Fatal("incorrect expected/approved/remaining quantity or item metadata")
			}
		}
		id, err := pkg.ParseID(document.ID)
		if err != nil {
			t.Fatal(err)
		}
		var audit struct {
			ActorID       uuid.UUID
			EventType     string
			AfterData     string
			CreatedAt     time.Time
			TransactionID *uuid.UUID `gorm:"column:receiving_transaction_id"`
		}
		if err := f.db.Table("receiving_audit_logs").Where("receiving_document_id = ?", id).Scan(&audit).Error; err != nil {
			t.Fatal(err)
		}
		var snapshot receiving.ReceivingResponse
		if json.Unmarshal([]byte(audit.AfterData), &snapshot) != nil || snapshot.ID != document.ID || audit.ActorID != f.users[pkg.Operator].ID || audit.EventType != "RECEIVING_CREATED" || audit.TransactionID != nil || !audit.CreatedAt.Equal(document.CreatedAt) {
			t.Fatal("missing atomic creation audit with public IDs")
		}
		workflowError(t, f.request("POST", "/api/v1/receivings", payload, pkg.Operator), 409, "already_exists")
		itemID, _ := pkg.ParseID(document.Items[0].ID)
		if err := f.db.Model(&receiving.DocumentItemStorage{}).Where("id = ?", itemID).Update("expected_quantity", 999).Error; err == nil {
			t.Fatal("database allowed expected snapshot mutation")
		}
	})
	t.Run("receiving_list_detail_status_and_validation", func(t *testing.T) {
		for _, role := range []string{pkg.Operator, pkg.Supervisor} {
			detail := responseData[receiving.ReceivingResponse](t, f.request("GET", "/api/v1/receivings/"+document.ID, nil, role))
			if detail.ID != document.ID || detail.Status != "INCOMPLETE" || len(detail.Items) != 2 {
				t.Fatal("incorrect document detail")
			}
			for i, item := range detail.Items {
				if item.ExpectedQuantity != document.Items[i].ExpectedQuantity {
					t.Fatal("stored expected snapshot changed")
				}
			}
			page := responseData[receiving.PaginatedReceivingResponse](t, f.request("GET", "/api/v1/receivings?status=INCOMPLETE&limit=1", nil, role))
			if len(page.Receivings) != 1 || page.Receivings[0].ID != document.ID || page.Meta != (pkg.PaginationMeta{CurrentPage: 1, TotalPages: 1, PageSize: 1, TotalData: 1}) {
				t.Fatal("incorrect receiving summary page")
			}
			completed := responseData[receiving.PaginatedReceivingResponse](t, f.request("GET", "/api/v1/receivings?status=COMPLETED", nil, role))
			outside := responseData[receiving.PaginatedReceivingResponse](t, f.request("GET", "/api/v1/receivings?page=10000", nil, role))
			if completed.Receivings == nil || len(completed.Receivings) != 0 || completed.Meta.TotalData != 0 || outside.Receivings == nil || len(outside.Receivings) != 0 || outside.Meta.TotalData != 1 {
				t.Fatal("incorrect receiving filter or empty/out-of-range page")
			}
		}
		workflowError(t, f.request("GET", "/api/v1/receivings/"+document.ID, nil, ""), 401, "unauthorized")
		for _, query := range []string{"status=", "status=REJECTED_BY_SYSTEM", "status=incomplete", "page=0", "page=10001", "limit=101"} {
			workflowError(t, f.request("GET", "/api/v1/receivings?"+query, nil, pkg.Operator), 400, "invalid_input")
		}
		for _, id := range []string{uuid.NewString(), document.ID + "==", "invalid"} {
			workflowError(t, f.request("GET", "/api/v1/receivings/"+id, nil, pkg.Operator), 400, "invalid_input")
		}
		workflowError(t, f.request("GET", "/api/v1/receivings/"+pkg.EncodeID(uuid.New()), nil, pkg.Operator), 404, "not_found")
	})
	t.Run("invalid_items_quantities_and_strict_binding", func(t *testing.T) {
		bodies := []string{
			`null`, `[]`, `{"order_reference":" "}`, `{"order_reference":"ORD-X"}`, `{"order_reference":"ORD-X","items":[]}`, `{"order_reference":"ORD-X","items":null}`, `{"order_reference":"ORD-X","items":[null]}`,
			`{"order_reference":"ORD-X","items":[{"expected_quantity":1}]}`,
			fmt.Sprintf(`{"order_reference":"ORD-X","items":[{"product_id":%q}]}`, productA.ID),
			fmt.Sprintf(`{"order_reference":"ORD-X","items":[{"product_id":%q,"expected_quantity":1}]}`, uuid.NewString()),
			fmt.Sprintf(`{"order_reference":"ORD-X","items":[{"product_id":%q,"expected_quantity":1},{"product_id":%q,"expected_quantity":2}]}`, productA.ID, productA.ID),
			fmt.Sprintf(`{"order_reference":"ORD-X","items":[{"product_id":%q,"expected_quantity":1}],"created_by":%q}`, productA.ID, pkg.EncodeID(f.users[pkg.Supervisor].ID)),
			fmt.Sprintf(`{"order_reference":"ORD-X","items":[{"product_id":%q,"expected_quantity":1,"approved_quantity":1}]}`, productA.ID),
		}
		for _, quantity := range []string{"0", "-1", "1.5", "9223372036854775808", `"1"`, "null"} {
			bodies = append(bodies, fmt.Sprintf(`{"order_reference":"ORD-X","items":[{"product_id":%q,"expected_quantity":%s}]}`, productA.ID, quantity))
		}
		for _, body := range bodies {
			workflowError(t, f.request("POST", "/api/v1/receivings", body, pkg.Operator), 400, "invalid_input")
		}
		workflowError(t, f.request("POST", "/api/v1/receivings", `{"order_reference":"`+strings.Repeat("x", 1<<20)+`"}`, pkg.Operator), 413, "payload_too_large")
		if tableCount(t, f.db, "receiving_documents") != 1 || tableCount(t, f.db, "receiving_document_items") != 2 || tableCount(t, f.db, "receiving_audit_logs") != 1 {
			t.Fatal("invalid requests partially wrote receiving data")
		}
	})
	t.Run("unknown_product_rolls_back_document_and_first_item", func(t *testing.T) {
		payload := receiving.CreateReceivingRequest{OrderReference: "ORD-MISSING", Items: []receiving.ExpectedItemRequest{
			{ProductID: productA.ID, ExpectedQuantity: 1}, {ProductID: pkg.EncodeID(uuid.New()), ExpectedQuantity: 1},
		}}
		workflowError(t, f.request("POST", "/api/v1/receivings", payload, pkg.Operator), 400, "invalid_input")
		if tableCount(t, f.db, "receiving_documents") != 1 || tableCount(t, f.db, "receiving_document_items") != 2 || tableCount(t, f.db, "receiving_audit_logs") != 1 {
			t.Fatal("foreign-key failure left partial document/items/audit")
		}
	})
	t.Run("audit_failure_rolls_back_entire_creation", func(t *testing.T) {
		now := time.Now().UTC()
		productID, _ := pkg.ParseID(productA.ID)
		id, missingActor := uuid.New(), uuid.New()
		doc := &domain.DocumentEntity{
			ID: id, OrderReference: "ORD-AUDIT-FAIL", Status: domain.Incomplete, CreatedBy: f.users[pkg.Operator].ID, CreatedAt: now, UpdatedAt: now,
			Items: []domain.DocumentItemEntity{{ID: uuid.New(), ReceivingDocumentID: id, ProductID: productID, ExpectedQuantity: 5}},
		}
		err := receiving.NewReceivingRepository(f.db).CreateDocument(context.Background(), doc, domain.AuditEntity{DocumentID: id, ActorID: &missingActor, EventType: "RECEIVING_CREATED", CreatedAt: now})
		if !errors.Is(err, pkg.ErrInvalidInput) {
			t.Fatalf("audit foreign-key failure error = %v", err)
		}
		if tableCount(t, f.db, "receiving_documents") != 1 || tableCount(t, f.db, "receiving_document_items") != 2 || tableCount(t, f.db, "receiving_audit_logs") != 1 {
			t.Fatal("audit failure left partial document/items/audit")
		}
	})
	t.Run("int64_expected_snapshot_round_trip", func(t *testing.T) {
		payload := receiving.CreateReceivingRequest{OrderReference: "ORD-INT64", Items: []receiving.ExpectedItemRequest{{ProductID: productA.ID, ExpectedQuantity: 9223372036854775807}}}
		response := f.request("POST", "/api/v1/receivings", payload, pkg.Operator)
		if response.Code != 201 {
			t.Fatalf("int64 quantity create status = %d", response.Code)
		}
		doc := responseData[receiving.ReceivingResponse](t, response)
		if doc.Items[0].ExpectedQuantity != 9223372036854775807 || doc.Items[0].RemainingQuantity != 9223372036854775807 {
			t.Fatal("expected quantity lost int64 precision")
		}
	})
	t.Run("concurrent_uniqueness", func(t *testing.T) {
		for _, kind := range []string{"products", "receivings"} {
			var payload any = products.CreateProductRequest{SKU: "SKU-RACE", Name: "Concurrent SKU"}
			role := pkg.Supervisor
			if kind == "receivings" {
				payload = receiving.CreateReceivingRequest{OrderReference: "ORD-RACE", Items: []receiving.ExpectedItemRequest{{ProductID: productA.ID, ExpectedQuantity: 1}}}
				role = pkg.Operator
			}
			start := make(chan struct{})
			responses := make(chan *httptest.ResponseRecorder, 4)
			var wg sync.WaitGroup
			for range 4 {
				wg.Go(func() { <-start; responses <- f.request("POST", "/api/v1/"+kind, payload, role) })
			}
			close(start)
			wg.Wait()
			close(responses)
			successes := 0
			for response := range responses {
				if response.Code == 201 {
					successes++
				} else {
					workflowError(t, response, 409, "already_exists")
				}
			}
			if successes != 1 {
				t.Fatalf("%s concurrent successes = %d, want 1", kind, successes)
			}
		}
	})
	t.Run("document_creation_does_not_post_inventory_or_deliver_notifications", func(t *testing.T) {
		for _, path := range []string{"/api/v1/notifications/unknown", "/api/v1/users/me"} {
			for _, method := range []string{"GET", "POST"} {
				workflowError(t, f.request(method, path, `{}`, pkg.Supervisor), 404, "not_found")
			}
		}
		for _, table := range []string{"receiving_transactions", "receiving_transaction_items", "inventory_balances", "inventory_transactions", "notification_outbox", "notifications"} {
			if tableCount(t, f.db, table) != 0 {
				t.Fatalf("excluded table %s was written", table)
			}
		}
	})
}

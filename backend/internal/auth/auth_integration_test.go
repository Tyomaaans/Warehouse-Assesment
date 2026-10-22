package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"warehouse/internal/auth/domain"
	"warehouse/internal/auth/session"
	"warehouse/internal/auth/token"
	"warehouse/internal/auth/user"
	"warehouse/internal/infrastructure/postgres"
	"warehouse/internal/inventory"
	"warehouse/internal/middleware"
	"warehouse/internal/products"
	"warehouse/internal/receiving"
	"warehouse/internal/router"
	"warehouse/migrations"
	"warehouse/pkg"
)

type authFixture struct {
	router   *gin.Engine
	db       *gorm.DB
	redis    *redis.Client
	operator user.UserStorage
	password string
	emails   map[string]string
}

type authLogin struct {
	access  string
	cookies []*http.Cookie
	sid     uuid.UUID
	refresh string
}

func newAuthFixture(t *testing.T) *authFixture {
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
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal("create isolated test schema:", err)
	}
	t.Cleanup(func() { _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error })
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL")
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
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal("invalid TEST_REDIS_URL")
	}
	options.DialTimeout, options.ReadTimeout, options.WriteTimeout = time.Second, time.Second, time.Second
	rc := redis.NewClient(options)
	t.Cleanup(func() { _ = rc.Close() })
	if err := rc.Ping(ctx).Err(); err != nil {
		t.Fatal("Redis:", err)
	}
	f := &authFixture{db: db, redis: rc, password: "task2b-test-password", emails: map[string]string{
		pkg.Operator: uuid.NewString() + "@example.test", pkg.Supervisor: uuid.NewString() + "@example.test",
	}}
	accounts := []user.ProvisionAccount{
		{Email: f.emails[pkg.Operator], Password: f.password, Role: pkg.Operator},
		{Email: f.emails[pkg.Supervisor], Password: f.password, Role: pkg.Supervisor},
	}
	if count, err := user.ProvisionUsers(ctx, db, pkg.NewValidator(), accounts); err != nil || count != 2 {
		t.Fatalf("ProvisionUsers() count = %d, error = %v", count, err)
	}
	if count, err := user.ProvisionUsers(ctx, db, pkg.NewValidator(), accounts); err != nil || count != 0 {
		t.Fatalf("repeated ProvisionUsers() count = %d, error = %v", count, err)
	}
	if err := db.Where("email = ?", f.emails[pkg.Operator]).First(&f.operator).Error; err != nil {
		t.Fatal(err)
	}
	repo := user.NewUserRepository(db)
	tokenSvc := token.NewJWTService("task2b-integration-signing-secret-32-bytes", 168*time.Hour, time.Hour, rc, session.NewSessionService(rc), repo)
	userSvc := user.NewUserService(repo, tokenSvc, pkg.NewValidator())
	authMiddleware := middleware.NewAuthMiddleware(tokenSvc)
	gin.SetMode(gin.ReleaseMode)
	f.router, err = router.NewRouter(zerolog.Nop(), func(context.Context) error { return nil }, nil, user.NewUserHandler(userSvc, true), authMiddleware,
		products.NewProductHandler(products.NewProductService(products.NewProductRepository(db), pkg.NewValidator())),
		receiving.NewReceivingHandler(receiving.NewReceivingService(receiving.NewReceivingRepository(db), pkg.NewValidator())),
		inventory.NewInventoryHandler(inventory.NewInventoryService(inventory.NewInventoryRepository(db))))
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{pkg.Operator, pkg.Supervisor} {
		f.router.GET("/test/"+role, authMiddleware.Authenticate(), authMiddleware.RequireRole(role), func(c *gin.Context) {
			if _, err := pkg.RequireActor(c.Request.Context(), role); err != nil {
				pkg.HTTPError(c, err)
				return
			}
			pkg.SuccessResponse(c, 200, "", gin.H{"allowed": true})
		})
	}
	return f
}

func (f *authFixture) request(method, path, body, access string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "https://warehouse.test"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, req)
	return response
}

func (f *authFixture) login(t *testing.T, role string, remember bool) authLogin {
	t.Helper()
	body, _ := json.Marshal(user.LoginRequest{Email: f.emails[role], Password: f.password, RememberMe: user.RememberMe(remember)})
	response := f.request("POST", "/api/v1/auth/login", string(body), "", nil)
	if response.Code != 200 {
		t.Fatalf("login status = %d, want 200", response.Code)
	}
	var envelope struct {
		Success bool               `json:"success"`
		Data    user.LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || !envelope.Success || envelope.Data.ExpiresIn != 900 {
		t.Fatal("invalid login response envelope or expiry")
	}
	id, err := pkg.ParseID(envelope.Data.User.ID)
	if err != nil || id == uuid.Nil || envelope.Data.User.Role != role || strings.Contains(response.Body.String(), "password") || strings.Contains(response.Body.String(), "refresh_token") {
		t.Fatal("invalid public user ID/role or credential disclosure")
	}
	result := authLogin{access: envelope.Data.AccessToken, cookies: response.Result().Cookies()}
	if len(result.cookies) != 2 || len(response.Header().Values("Set-Cookie")) != 2 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("expected two cookie headers and no-store")
	}
	maxAge := 3600
	if remember {
		maxAge = 604800
	}
	for _, cookie := range result.cookies {
		if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.MaxAge != maxAge {
			t.Fatal("incorrect session cookie attributes")
		}
		switch cookie.Name {
		case "refresh_token":
			if cookie.Path != "/api/v1/auth" || len(cookie.Value) != 64 {
				t.Fatal("invalid refresh cookie path/token")
			}
			result.refresh = cookie.Value
		case "session_id":
			if cookie.Path != "/api/v1" {
				t.Fatal("invalid session cookie path")
			}
			result.sid, err = pkg.ParseID(cookie.Value)
			if err != nil {
				t.Fatal("noncanonical session cookie ID")
			}
		}
	}
	t.Cleanup(func() {
		ctx := context.Background()
		key := "session:" + session.HashID(result.sid.String())
		data, _ := f.redis.Get(ctx, key).Bytes()
		var sess domain.SessionEntity
		_ = json.Unmarshal(data, &sess)
		_ = f.redis.Del(ctx, key, "refresh:"+sess.RefreshToken, "refresh:"+session.HashID(result.refresh), "user_sessions:"+id.String()).Err()
	})
	return result
}

func wantError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var envelope struct {
		Success bool            `json:"success"`
		Error   pkg.ErrorDetail `json:"error"`
	}
	if response.Code != status || json.Unmarshal(response.Body.Bytes(), &envelope) != nil || envelope.Success || envelope.Error.Code != code || envelope.Error.Message == "" {
		t.Fatalf("status/code = %d/%s, want %d/%s", response.Code, envelope.Error.Code, status, code)
	}
}

func TestAuthenticationIntegration(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	t.Run("invalid_credentials_and_strict_binding", func(t *testing.T) {
		for _, email := range []string{f.emails[pkg.Operator], "missing@example.test"} {
			body := fmt.Sprintf(`{"email":%q,"password":"incorrect"}`, email)
			wantError(t, f.request("POST", "/api/v1/auth/login", body, "", nil), 401, "invalid_credentials")
		}
		for _, body := range []string{`null`, `{"email":"invalid","password":"p"}`, `{"email":"a@example.test","password":"p","role":"SUPERVISOR"}`, `{"email":"a@example.test","password":"p"}{}`, `{"email":"a@example.test","password":"p","remember_me":null}`} {
			wantError(t, f.request("POST", "/api/v1/auth/login", body, "", nil), 400, "invalid_input")
		}
		wantError(t, f.request("POST", "/api/v1/auth/login", `{"password":"`+strings.Repeat("x", 1<<20)+`"}`, "", nil), 413, "payload_too_large")
	})
	t.Run("revocation_wins_before_rotation_commit", func(t *testing.T) {
		login := f.login(t, pkg.Operator, false)
		sessionSvc := session.NewSessionService(f.redis)
		sess, err := sessionSvc.GetSession(ctx, login.sid.String())
		if err != nil {
			t.Fatal(err)
		}
		oldHash := session.HashID(login.refresh)
		payload, err := sessionSvc.GetRefreshToken(ctx, oldHash)
		if err != nil {
			t.Fatal(err)
		}
		lockKey, owner := "lock:refresh:"+oldHash, uuid.NewString()
		if err := f.redis.Set(ctx, lockKey, owner, 5*time.Second).Err(); err != nil {
			t.Fatal(err)
		}
		defer f.redis.Del(ctx, lockKey)
		sess.RefreshToken = session.HashID(uuid.NewString())
		if err := sessionSvc.RevokeSession(ctx, sess.UserID, sess.SessionID, oldHash); err != nil {
			t.Fatal(err)
		}
		err = sessionSvc.RotateSession(ctx, sess, payload, oldHash, owner)
		if !errors.Is(err, pkg.ErrUnauthorized) {
			t.Fatalf("rotation after revocation error = %v, want unauthorized", err)
		}
		if count, _ := f.redis.Exists(ctx, "session:"+session.HashID(sess.SessionID), "refresh:"+sess.RefreshToken).Result(); count != 0 {
			t.Fatal("rotation recreated revoked credentials")
		}
	})
	t.Run("redis_failure_fails_closed", func(t *testing.T) {
		login := f.login(t, pkg.Operator, false)
		closedRedis := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
		_ = closedRedis.Close()
		repo := user.NewUserRepository(f.db)
		tokenSvc := token.NewJWTService("task2b-integration-signing-secret-32-bytes", 168*time.Hour, time.Hour, closedRedis, session.NewSessionService(closedRedis), repo)
		userSvc := user.NewUserService(repo, tokenSvc, pkg.NewValidator())
		authMiddleware := middleware.NewAuthMiddleware(tokenSvc)
		r, err := router.NewRouter(zerolog.Nop(), func(context.Context) error { return nil }, nil, user.NewUserHandler(userSvc, true), authMiddleware,
			products.NewProductHandler(products.NewProductService(products.NewProductRepository(f.db), pkg.NewValidator())),
			receiving.NewReceivingHandler(receiving.NewReceivingService(receiving.NewReceivingRepository(f.db), pkg.NewValidator())),
			inventory.NewInventoryHandler(inventory.NewInventoryService(inventory.NewInventoryRepository(f.db))))
		if err != nil {
			t.Fatal(err)
		}
		r.GET("/test/OPERATOR", authMiddleware.Authenticate(), func(c *gin.Context) { c.Status(200) })
		outage := &authFixture{router: r}
		wantError(t, outage.request("GET", "/test/OPERATOR", "", login.access, nil), 503, "unavailable")
		wantError(t, outage.request("POST", "/api/v1/auth/refresh", "", "", login.cookies), 503, "unavailable")
		body, _ := json.Marshal(user.LoginRequest{Email: f.emails[pkg.Operator], Password: f.password})
		wantError(t, outage.request("POST", "/api/v1/auth/login", string(body), "", nil), 503, "unavailable")
	})
	t.Run("role_isolation_and_service_actor", func(t *testing.T) {
		for _, role := range []string{pkg.Operator, pkg.Supervisor} {
			login := f.login(t, role, false)
			for _, required := range []string{pkg.Operator, pkg.Supervisor} {
				response := f.request("GET", "/test/"+required, "", login.access, nil)
				if role != required {
					wantError(t, response, 403, "forbidden")
				} else if response.Code != 200 {
					t.Fatalf("own role status = %d, want 200", response.Code)
				}
			}
		}
		wantError(t, f.request("GET", "/test/OPERATOR", "", "", nil), 401, "unauthorized")
	})
	t.Run("hashed_storage_rotation_replay_and_lock", func(t *testing.T) {
		login := f.login(t, pkg.Operator, true)
		hash := session.HashID(login.refresh)
		sessionKey := "session:" + session.HashID(login.sid.String())
		for _, key := range []string{sessionKey, "refresh:" + hash} {
			data, err := f.redis.Get(ctx, key).Result()
			if err != nil || strings.Contains(data, login.refresh) {
				t.Fatal("missing session or raw refresh token persisted")
			}
			ttl, err := f.redis.TTL(ctx, key).Result()
			if err != nil || ttl < 167*time.Hour || ttl > 168*time.Hour {
				t.Fatal("remember_me Redis/cookie TTL mismatch")
			}
		}
		lockKey := "lock:refresh:" + hash
		if err := f.redis.Set(ctx, lockKey, "another-owner", 5*time.Second).Err(); err != nil {
			t.Fatal(err)
		}
		wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", login.cookies), 409, "concurrent_refresh")
		if owner, _ := f.redis.Get(ctx, lockKey).Result(); owner != "another-owner" {
			t.Fatal("refresh removed another owner's lock")
		}
		_ = f.redis.Del(ctx, lockKey).Err()
		response := f.request("POST", "/api/v1/auth/refresh", "", "", login.cookies)
		if response.Code != 200 || strings.Contains(response.Body.String(), "refresh_token") {
			t.Fatalf("rotation status = %d, want 200 without refresh JSON", response.Code)
		}
		var refresh struct {
			Success bool                 `json:"success"`
			Data    user.RefreshResponse `json:"data"`
		}
		if json.Unmarshal(response.Body.Bytes(), &refresh) != nil || !refresh.Success || refresh.Data.ExpiresIn != 900 {
			t.Fatal("invalid refresh envelope")
		}
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == "session_id" && cookie.Value != pkg.EncodeID(login.sid) {
				t.Fatal("rotation changed session ID")
			}
			if cookie.Name == "refresh_token" && cookie.Value == login.refresh {
				t.Fatal("refresh token was not rotated")
			}
			if cookie.MaxAge != 604800 || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
				t.Fatal("rotation changed cookie attributes or TTL")
			}
		}
		wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", login.cookies), 401, "unauthorized")
		if n, _ := f.redis.Exists(ctx, "refresh:"+hash).Result(); n != 0 {
			t.Fatal("used refresh token still exists")
		}
	})
	t.Run("cookie_pair_and_bearer_session_binding", func(t *testing.T) {
		first, second := f.login(t, pkg.Operator, false), f.login(t, pkg.Supervisor, false)
		wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", nil), 401, "unauthorized")
		mixed := []*http.Cookie{{Name: "refresh_token", Value: first.refresh}, {Name: "session_id", Value: pkg.EncodeID(second.sid)}}
		wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", mixed), 401, "unauthorized")
		wantError(t, f.request("POST", "/api/v1/auth/logout", "", first.access, second.cookies), 401, "unauthorized")
		malformed := []*http.Cookie{{Name: "refresh_token", Value: first.refresh}, {Name: "session_id", Value: first.sid.String()}}
		wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", malformed), 400, "invalid_input")
		if response := f.request("GET", "/test/SUPERVISOR", "", second.access, nil); response.Code != 200 {
			t.Fatal("mismatched logout revoked the other session")
		}
	})
	t.Run("logout_revokes_access_and_refresh", func(t *testing.T) {
		login := f.login(t, pkg.Operator, false)
		wantError(t, f.request("POST", "/api/v1/auth/logout", "", login.access, nil), 401, "unauthorized")
		response := f.request("POST", "/api/v1/auth/logout", "", login.access, login.cookies)
		if response.Code != 204 || response.Body.Len() != 0 || len(response.Result().Cookies()) != 2 {
			t.Fatal("logout must return empty 204 and two clearing cookies")
		}
		for _, cookie := range response.Result().Cookies() {
			expectedPath := "/api/v1"
			if cookie.Name == "refresh_token" {
				expectedPath += "/auth"
			}
			if cookie.MaxAge != -1 || cookie.Value != "" || cookie.Path != expectedPath || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" {
				t.Fatal("incorrect clearing cookie")
			}
		}
		if n, _ := f.redis.Exists(ctx, "session:"+session.HashID(login.sid.String()), "refresh:"+session.HashID(login.refresh)).Result(); n != 0 {
			t.Fatal("logout did not remove session and refresh records")
		}
		wantError(t, f.request("GET", "/test/OPERATOR", "", login.access, nil), 401, "unauthorized")
		wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", login.cookies), 401, "unauthorized")
	})
	t.Run("expired_access_refresh_and_session", func(t *testing.T) {
		login := f.login(t, pkg.Operator, false)
		expired, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, gojwt.MapClaims{
			"sub": f.operator.ID.String(), "sid": login.sid.String(), "role": pkg.Operator,
			"iat": time.Now().Add(-time.Hour).Unix(), "exp": time.Now().Add(-45 * time.Minute).Unix(),
		}).SignedString([]byte("task2b-integration-signing-secret-32-bytes"))
		if err != nil {
			t.Fatal(err)
		}
		wantError(t, f.request("GET", "/test/OPERATOR", "", expired, nil), 401, "unauthorized")
		if err := f.redis.PExpire(ctx, "refresh:"+session.HashID(login.refresh), -time.Second).Err(); err != nil {
			t.Fatal(err)
		}
		wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", login.cookies), 401, "unauthorized")
		if err := f.redis.PExpire(ctx, "session:"+session.HashID(login.sid.String()), -time.Second).Err(); err != nil {
			t.Fatal(err)
		}
		wantError(t, f.request("GET", "/test/OPERATOR", "", login.access, nil), 401, "unauthorized")
	})
	t.Run("changed_database_role_invalidates_auth", func(t *testing.T) {
		login := f.login(t, pkg.Operator, false)
		if err := f.db.Model(&user.UserStorage{}).Where("id = ?", f.operator.ID).Update("role", pkg.Supervisor).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = f.db.Model(&user.UserStorage{}).Where("id = ?", f.operator.ID).Update("role", pkg.Operator).Error
		})
		wantError(t, f.request("GET", "/test/OPERATOR", "", login.access, nil), 401, "unauthorized")
		wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", login.cookies), 401, "unauthorized")
	})
	t.Run("concurrent_refresh_is_single_use", func(t *testing.T) {
		login := f.login(t, pkg.Operator, false)
		start := make(chan struct{})
		responses := make(chan *httptest.ResponseRecorder, 8)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() { <-start; responses <- f.request("POST", "/api/v1/auth/refresh", "", "", login.cookies) })
		}
		close(start)
		wg.Wait()
		close(responses)
		successes := 0
		for response := range responses {
			switch response.Code {
			case 200:
				successes++
			case 401:
				wantError(t, response, 401, "unauthorized")
			case 409:
				wantError(t, response, 409, "concurrent_refresh")
			default:
				t.Fatalf("concurrent refresh status = %d", response.Code)
			}
		}
		if successes != 1 {
			t.Fatalf("successful rotations = %d, want 1", successes)
		}
	})
	t.Run("logout_refresh_race_cannot_restore_revoked_session", func(t *testing.T) {
		for range 8 {
			login := f.login(t, pkg.Operator, false)
			start := make(chan struct{})
			var refresh, logout *httptest.ResponseRecorder
			var wg sync.WaitGroup
			wg.Go(func() { <-start; refresh = f.request("POST", "/api/v1/auth/refresh", "", "", login.cookies) })
			wg.Go(func() { <-start; logout = f.request("POST", "/api/v1/auth/logout", "", login.access, login.cookies) })
			close(start)
			wg.Wait()
			if logout.Code == 401 && refresh.Code == 200 {
				logout = f.request("POST", "/api/v1/auth/logout", "", login.access, refresh.Result().Cookies())
			}
			if logout.Code != 204 {
				t.Fatalf("race logout/refresh status = %d/%d", logout.Code, refresh.Code)
			}
			wantError(t, f.request("GET", "/test/OPERATOR", "", login.access, nil), 401, "unauthorized")
			if refresh.Code == 200 {
				wantError(t, f.request("POST", "/api/v1/auth/refresh", "", "", refresh.Result().Cookies()), 401, "unauthorized")
			}
		}
	})
	t.Run("unfinished_workflows_remain_unregistered", func(t *testing.T) {
		for _, path := range []string{"/api/v1/notifications", "/api/v1/transactions", "/api/v1/users/me", "/api/v1/auth/register"} {
			wantError(t, f.request("GET", path, "", "", nil), 404, "not_found")
		}
	})
}

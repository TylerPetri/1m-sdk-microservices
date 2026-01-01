//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	hellov1 "sdk-microservices/gen/api/proto/hello/v1"
	"sdk-microservices/internal/db"
	authsrv "sdk-microservices/internal/services/auth/server"
	"sdk-microservices/internal/services/auth/jwt"
	"sdk-microservices/internal/services/auth/store"
	hellosrv "sdk-microservices/internal/services/hello/server"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestIntegration_MigrationsAndSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pg := startPostgres(t, ctx)
	dsn := mustConnString(t, ctx, pg)

	pool := mustPool(t, ctx, dsn)
	defer pool.Close()

	applyAuthMigrations(t, ctx, pool)

	// smoke query: ensure users table exists
	var ok bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.users') IS NOT NULL`).Scan(&ok); err != nil {
		t.Fatalf("smoke query err=%v", err)
	}
	if !ok {
		t.Fatalf("users table missing after migrations")
	}
}

func TestIntegration_gRPC_and_HTTP_Smoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pg := startPostgres(t, ctx)
	dsn := mustConnString(t, ctx, pg)

	pool := mustPool(t, ctx, dsn)
	defer pool.Close()

	applyAuthMigrations(t, ctx, pool)

	// Start auth gRPC.
	authAddr, authStop := startAuthGRPC(t, ctx, pool)
	defer authStop()

	// Start hello gRPC.
	helloAddr, helloStop := startHelloGRPC(t, ctx)
	defer helloStop()

	// Start gateway HTTP.
	gatewayURL, gwStop := startGatewayHTTP(t, ctx, helloAddr, authAddr)
	defer gwStop()

	// Register via HTTP.
	email := fmt.Sprintf("u_%d@example.com", time.Now().UnixNano())
	password := "supersecurepassword" // >= 12
	reg := map[string]any{"email": email, "password": password}
	regBody := mustJSON(t, reg)
	resp := mustHTTP(t, ctx, "POST", gatewayURL+"/v1/auth/register", regBody, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register status=%d body=%s", resp.StatusCode, mustReadAll(t, resp.Body))
	}
	_ = resp.Body.Close()

	// Login via HTTP.
	loginBody := mustJSON(t, map[string]any{"email": email, "password": password})
	resp = mustHTTP(t, ctx, "POST", gatewayURL+"/v1/auth/login", loginBody, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", resp.StatusCode, mustReadAll(t, resp.Body))
	}
	var lr struct {
		UserID      string `json:"userId"`
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode login err=%v", err)
	}
	_ = resp.Body.Close()
	if lr.UserID == "" || lr.AccessToken == "" {
		t.Fatalf("login response missing fields: %+v", lr)
	}

	// Call hello via HTTP with bearer token (gateway protects non-auth endpoints).
	headers := map[string]string{"Authorization": "Bearer " + lr.AccessToken}
	resp = mustHTTP(t, ctx, "GET", gatewayURL+"/v1/hello/tyler", nil, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("hello status=%d body=%s", resp.StatusCode, mustReadAll(t, resp.Body))
	}
	_ = resp.Body.Close()
}

func TestContract_gRPC_StatusCodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pg := startPostgres(t, ctx)
	dsn := mustConnString(t, ctx, pg)
	pool := mustPool(t, ctx, dsn)
	defer pool.Close()
	applyAuthMigrations(t, ctx, pool)

	authAddr, stop := startAuthGRPC(t, ctx, pool)
	defer stop()

	conn, err := grpc.DialContext(ctx, authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial auth err=%v", err)
	}
	defer conn.Close()

	c := authv1.NewAuthServiceClient(conn)

	// Invalid email -> InvalidArgument.
	_, err = c.Register(ctx, &authv1.RegisterRequest{Email: "nope", Password: "supersecurepassword"})
	if st := status.Convert(err); st == nil || st.Code() == 0 {
		t.Fatalf("expected gRPC error")
	} else if st.Code().String() != "InvalidArgument" {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}

	// Login unknown user -> Unauthenticated (avoid enumeration).
	_, err = c.Login(ctx, &authv1.LoginRequest{Email: "nobody@example.com", Password: "supersecurepassword"})
	if st := status.Convert(err); st == nil {
		t.Fatalf("expected gRPC error")
	} else if st.Code().String() != "Unauthenticated" {
		t.Fatalf("expected Unauthenticated, got %v", st.Code())
	}
}

// --- helpers ---

func startPostgres(t *testing.T, ctx context.Context) *postgres.PostgresContainer {
	t.Helper()
	pg, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase("auth"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
	)
	if err != nil {
		t.Fatalf("start postgres err=%v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	return pg
}

func mustConnString(t *testing.T, ctx context.Context, pg *postgres.PostgresContainer) string {
	t.Helper()
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("conn string err=%v", err)
	}
	return dsn
}

func mustPool(t *testing.T, ctx context.Context, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := db.NewPool(ctx, dsn, db.Options{InitialPingTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewPool err=%v", err)
	}
	return pool
}

func applyAuthMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root := projectRoot(t)
	migDir := filepath.Join(root, "migrations", "auth")
	ents, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatalf("ReadDir migrations err=%v", err)
	}
	var files []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".sql" {
			continue
		}
		if filepath.Base(name) == name && filepath.Ext(name) == ".sql" {
			// ok
		}
		if filepath.Base(name) != name {
			continue
		}
		if filepath.Ext(name) == ".sql" && filepathHasSuffix(name, ".up.sql") {
			files = append(files, filepath.Join(migDir, name))
		}
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("ReadFile %s err=%v", f, err)
		}
		if _, err := pool.Exec(ctx, string(b)); err != nil {
			t.Fatalf("exec migration %s err=%v", f, err)
		}
	}
}

func filepathHasSuffix(name, suf string) bool {
	if len(suf) > len(name) {
		return false
	}
	return name[len(name)-len(suf):] == suf
}

func startAuthGRPC(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (addr string, stop func()) {
	t.Helper()
	st := store.New(pool)
	jwtSvc := jwt.New("test-secret", "sdk-microservices")
	srv := authsrv.New(zap.NewNop(), st, jwtSvc, authsrv.Options{AccessTTL: 2 * time.Minute, RefreshTTL: 10 * time.Minute})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen err=%v", err)
	}
	gs := grpc.NewServer()
	authv1.RegisterAuthServiceServer(gs, srv)

	go func() { _ = gs.Serve(lis) }()
	return lis.Addr().String(), func() {
		gs.Stop()
		_ = lis.Close()
	}
}

func startHelloGRPC(t *testing.T, ctx context.Context) (addr string, stop func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen err=%v", err)
	}
	gs := grpc.NewServer()
	hellov1.RegisterHelloServiceServer(gs, &hellosrv.Server{})
	go func() { _ = gs.Serve(lis) }()
	return lis.Addr().String(), func() {
		gs.Stop()
		_ = lis.Close()
	}
}

func startGatewayHTTP(t *testing.T, ctx context.Context, helloAddr, authAddr string) (baseURL string, stop func()) {
	t.Helper()
	// gRPC dials for handlers.
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	connHello, err := grpc.DialContext(dialCtx, helloAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial hello err=%v", err)
	}
	connAuth, err := grpc.DialContext(dialCtx, authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = connHello.Close()
		t.Fatalf("dial auth err=%v", err)
	}

	mux := runtime.NewServeMux()
	if err := hellov1.RegisterHelloServiceHandlerClient(ctx, mux, hellov1.NewHelloServiceClient(connHello)); err != nil {
		_ = connHello.Close()
		_ = connAuth.Close()
		t.Fatalf("register hello gw err=%v", err)
	}
	if err := authv1.RegisterAuthServiceHandlerClient(ctx, mux, authv1.NewAuthServiceClient(connAuth)); err != nil {
		_ = connHello.Close()
		_ = connAuth.Close()
		t.Fatalf("register auth gw err=%v", err)
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = connHello.Close()
		_ = connAuth.Close()
		t.Fatalf("listen http err=%v", err)
	}

	go func() { _ = srv.Serve(lis) }()
	return "http://" + lis.Addr().String(), func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		_ = connHello.Close()
		_ = connAuth.Close()
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test file's working dir until we find go.mod.
	d, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd err=%v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("could not locate project root from %s", d)
	return ""
}

func mustJSON(t *testing.T, v any) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal err=%v", err)
	}
	return bytes.NewReader(b)
}

func mustHTTP(t *testing.T, ctx context.Context, method, url string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		t.Fatalf("NewRequest err=%v", err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do err=%v", err)
	}
	return resp
}

func mustReadAll(t *testing.T, r io.Reader) string {
	t.Helper()
	b, _ := io.ReadAll(r)
	return string(b)
}

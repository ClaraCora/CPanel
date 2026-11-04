package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"cpanel/internal/database"
	"cpanel/internal/domain"
	"cpanel/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestExhaustedQuotaDoesNotHidePortalUsagePostgres(t *testing.T) {
	dsn := os.Getenv("CPANEL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CPANEL_TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := domain.MustID("portal_quota_test")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO access_groups(id,name) VALUES('quota_group','Quota');
		INSERT INTO plans(id,name,access_group_id,traffic_limit_bytes,reset_strategy) VALUES('quota_plan','Quota','quota_group',100,'never')`); err != nil {
		t.Fatal(err)
	}
	s := store.New(pool)
	plan := "quota_plan"
	created, err := s.CreateUser(ctx, domain.UserCreate{Role: "user", Name: "Quota", PlanID: &plan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET traffic_used_bytes=200 WHERE id=$1`, created.ID); err != nil {
		t.Fatal(err)
	}
	server := &Server{store: s}
	request := httptest.NewRequest(http.MethodGet, "/ca/edu/zl", nil).WithContext(context.WithValue(ctx, portalKey, domain.PortalSession{User: domain.User{ID: created.ID}}))
	response := httptest.NewRecorder()
	server.handlePortalDashboard(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data domain.PortalDashboard `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.TrafficUsedBytes != 200 || payload.Data.TrafficLimitBytes != 100 || len(payload.Data.Nodes) != 0 || payload.Data.AccessUnavailableReason == "" {
		t.Fatalf("dashboard=%+v", payload.Data)
	}
}

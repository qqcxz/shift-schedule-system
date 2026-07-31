package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestSQLiteSeedAndBusinessFlow(t *testing.T) {
	databasePath := fmt.Sprintf("%s/shift.sqlite", t.TempDir())
	service, err := New(context.Background(), Config{
		Port:               3000,
		DBType:             "sqlite",
		DBDatabase:         databasePath,
		JWTSecret:          "test-secret",
		CORSOrigins:        []string{"http://localhost:5173"},
		SeedOnBoot:         true,
		ScheduleKeepMonths: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	healthResponse, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	healthResponse.Body.Close()
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := healthResponse.Header.Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}

	managerToken := loginTestUser(t, server.URL, "qqcxz", "123456")
	staffToken := loginTestUser(t, server.URL, "staff1", "123456")

	var me User
	getJSONTest(t, server.URL+"/api/auth/me", managerToken, &me)
	if me.Username != "qqcxz" || me.Role != RoleManager {
		t.Fatalf("unexpected manager: %+v", me)
	}

	month := monthNow()
	var schedules struct {
		Month string         `json:"month"`
		Items []ScheduleItem `json:"items"`
	}
	getJSONTest(t, server.URL+"/api/schedules?month="+month, staffToken, &schedules)
	if len(schedules.Items) == 0 || len(schedules.Items[0].WorkDate) != 10 {
		t.Fatalf("expected YYYY-MM-DD schedule dates: %+v", schedules)
	}

	var shifts []ShiftTemplate
	getJSONTest(t, server.URL+"/api/shifts", staffToken, &shifts)
	if len(shifts) < 2 {
		t.Fatal("seed should create shifts")
	}

	requestBody := map[string]any{
		"type":      RequestChange,
		"fromDate":  schedules.Items[0].WorkDate,
		"toShiftId": shifts[0].ID,
		"reason":    "Go integration test",
	}
	var request ScheduleRequest
	postJSONTest(t, server.URL+"/api/requests", staffToken, requestBody, &request, http.StatusCreated)
	if request.Status != RequestPending || request.FromDate != schedules.Items[0].WorkDate {
		t.Fatalf("unexpected request: %+v", request)
	}
	assertStatusTest(t, server.URL+"/api/requests/"+request.ID+"/reject", managerToken, http.MethodPost, map[string]any{"action": "approve"}, http.StatusBadRequest)
	var approved ScheduleRequest
	postJSONTest(t, server.URL+"/api/requests/"+request.ID+"/approve", managerToken, map[string]any{"reviewNote": "ok"}, &approved, http.StatusCreated)
	if approved.Status != RequestApproved || approved.ReviewedAt == nil {
		t.Fatalf("request was not approved: %+v", approved)
	}

	var notifications []Notification
	getJSONTest(t, server.URL+"/api/notifications", staffToken, &notifications)
	if len(notifications) == 0 || notifications[0].Title != "申请已通过" {
		t.Fatalf("expected approval notification: %+v", notifications)
	}
	assertStatusTest(t, server.URL+"/api/requests/"+request.ID, managerToken, http.MethodDelete, nil, http.StatusOK)
	getJSONTest(t, server.URL+"/api/notifications", staffToken, &notifications)
	if len(notifications) != 0 {
		t.Fatalf("deleting a request should remove related notifications: %+v", notifications)
	}

	requestDate := schedules.Items[0].WorkDate
	var first ScheduleItem
	for _, candidate := range schedules.Items[1:] {
		if candidate.WorkDate != requestDate {
			first = candidate
			break
		}
	}
	if first.ID == "" {
		t.Fatal("seed should create schedules on at least two dates for optimistic-lock test")
	}
	saveBody := map[string]any{"month": month, "items": []any{map[string]any{
		"userId":          first.UserID,
		"workDate":        first.WorkDate,
		"shiftTemplateId": first.ShiftTemplateID,
		"version":         first.Version,
	}}}
	var saveResult map[string]any
	putJSONTest(t, server.URL+"/api/schedules/month", managerToken, saveBody, &saveResult, http.StatusOK)
	assertStatusTest(t, server.URL+"/api/schedules/month", managerToken, http.MethodPut, saveBody, http.StatusBadRequest)
}

func TestSecurityAndValidationBoundaries(t *testing.T) {
	databasePath := fmt.Sprintf("%s/shift.sqlite", t.TempDir())
	service, err := New(context.Background(), Config{
		Port:               3000,
		DBType:             "sqlite",
		DBDatabase:         databasePath,
		JWTSecret:          "test-secret",
		CORSOrigins:        []string{"https://schedule.example"},
		SeedOnBoot:         true,
		ScheduleKeepMonths: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	managerToken := loginTestUser(t, server.URL, "qqcxz", "123456")
	staffToken := loginTestUser(t, server.URL, "staff1", "123456")

	var users []User
	getJSONTest(t, server.URL+"/api/users", managerToken, &users)
	var staff User
	for _, user := range users {
		if user.Username == "staff1" {
			staff = user
			break
		}
	}
	if staff.ID == "" {
		t.Fatal("seed should create staff1")
	}

	var shifts []ShiftTemplate
	getJSONTest(t, server.URL+"/api/shifts", managerToken, &shifts)
	var schedules struct {
		Month string         `json:"month"`
		Items []ScheduleItem `json:"items"`
	}
	getJSONTest(t, server.URL+"/api/schedules?month="+monthNow(), managerToken, &schedules)
	if len(schedules.Items) == 0 {
		t.Fatal("seed should create schedules")
	}
	item := schedules.Items[0]

	assertStatusTest(t, server.URL+"/api/schedules?month=2026-13", managerToken, http.MethodGet, nil, http.StatusBadRequest)
	assertStatusTest(t, server.URL+"/api/schedules?month=0000-01", managerToken, http.MethodGet, nil, http.StatusBadRequest)
	assertStatusTest(t, server.URL+"/api/requests", staffToken, http.MethodPost, map[string]any{
		"type":     RequestLeave,
		"fromDate": "2026-02-30",
	}, http.StatusBadRequest)
	assertStatusTest(t, server.URL+"/api/requests", staffToken, http.MethodPost, map[string]any{
		"type":     RequestLeave,
		"fromDate": "2026-07-01",
		"toDate":   "2026-07-02",
	}, http.StatusBadRequest)
	assertStatusTest(t, server.URL+"/api/requests", staffToken, http.MethodPost, map[string]any{
		"type":     RequestLeave,
		"fromDate": "0000-07-01",
	}, http.StatusBadRequest)
	assertStatusTest(t, server.URL+"/api/schedules/month", managerToken, http.MethodPut, map[string]any{
		"month": schedules.Month,
		"items": []any{
			map[string]any{"userId": item.UserID, "workDate": item.WorkDate, "shiftTemplateId": item.ShiftTemplateID, "version": item.Version},
			map[string]any{"userId": item.UserID, "workDate": item.WorkDate, "shiftTemplateId": item.ShiftTemplateID, "version": item.Version},
		},
	}, http.StatusBadRequest)

	otherStoreID := uuid.NewString()
	otherUserID := uuid.NewString()
	now := nowDB()
	if _, err := service.db.ExecContext(context.Background(), "INSERT INTO `stores` (`id`, `name`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?)", otherStoreID, "Other Store", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := service.db.ExecContext(context.Background(), "INSERT INTO `users` (`id`, `username`, `displayName`, `passwordHash`, `role`, `storeId`, `isActive`, `authVersion`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)", otherUserID, "other-staff", "Other", "unused", RoleStaff, otherStoreID, true, now, now); err != nil {
		t.Fatal(err)
	}
	assertStatusTest(t, server.URL+"/api/requests", staffToken, http.MethodPost, map[string]any{
		"type":         RequestSwap,
		"fromDate":     item.WorkDate,
		"targetUserId": otherUserID,
	}, http.StatusBadRequest)

	patchJSONTest(t, server.URL+"/api/users/"+staff.ID, managerToken, map[string]any{"isActive": false}, http.StatusOK)
	assertStatusTest(t, server.URL+"/api/auth/me", staffToken, http.MethodGet, nil, http.StatusUnauthorized)

	postJSONTest(t, server.URL+"/api/auth/change-password", managerToken, map[string]any{
		"currentPassword": "123456",
		"newPassword":     "manager-pass-2026",
	}, &map[string]bool{}, http.StatusCreated)
	assertStatusTest(t, server.URL+"/api/auth/me", managerToken, http.MethodGet, nil, http.StatusUnauthorized)
	newManagerToken := loginTestUser(t, server.URL, "qqcxz", "manager-pass-2026")
	assertStatusTest(t, server.URL+"/api/auth/me", newManagerToken, http.MethodGet, nil, http.StatusOK)

	if err := validateShiftRange("22:00", "06:00", false); err == nil {
		t.Fatal("overnight shifts should be rejected until cross-day scheduling is implemented")
	}
	if _, _, err := monthRange("0000-01"); err == nil {
		t.Fatal("year zero should be rejected")
	}
}

func TestSQLiteMigrationAddsAuthVersionToLegacySchema(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "legacy.sqlite")
	legacyDB, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath))
	if err != nil {
		t.Fatal(err)
	}
	legacySchema := []string{
		`CREATE TABLE "stores" ("id" varchar PRIMARY KEY NOT NULL, "name" varchar NOT NULL, "address" varchar, "createdAt" datetime NOT NULL DEFAULT (datetime('now')), "updatedAt" datetime NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE "users" ("id" varchar PRIMARY KEY NOT NULL, "username" varchar NOT NULL, "displayName" varchar NOT NULL, "passwordHash" varchar NOT NULL, "role" varchar NOT NULL DEFAULT ('staff'), "storeId" varchar NOT NULL, "isActive" boolean NOT NULL DEFAULT (1), "createdAt" datetime NOT NULL DEFAULT (datetime('now')), "updatedAt" datetime NOT NULL DEFAULT (datetime('now')), CONSTRAINT "UQ_users_username" UNIQUE ("username"), CONSTRAINT "FK_users_store" FOREIGN KEY ("storeId") REFERENCES "stores" ("id") ON DELETE CASCADE);`,
	}
	for _, statement := range legacySchema {
		if _, err := legacyDB.Exec(statement); err != nil {
			legacyDB.Close()
			t.Fatal(err)
		}
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatal(err)
	}

	service, err := New(context.Background(), Config{DBType: "sqlite", DBDatabase: databasePath, JWTSecret: "test-secret", SeedOnBoot: false})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	var columnCount int
	if err := service.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'authVersion'").Scan(&columnCount); err != nil {
		t.Fatal(err)
	}
	if columnCount != 1 {
		t.Fatalf("expected authVersion migration, got %d columns", columnCount)
	}
	var migrationCount int
	if err := service.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version IN (1, 2)").Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 2 {
		t.Fatalf("expected migration versions 1 and 2, got %d", migrationCount)
	}
}

func loginTestUser(t *testing.T, base, username, password string) string {
	t.Helper()
	var response struct {
		AccessToken string `json:"accessToken"`
	}
	postJSONTest(t, base+"/api/auth/login", "", map[string]string{"username": username, "password": password}, &response, http.StatusCreated)
	if response.AccessToken == "" {
		t.Fatal("login returned an empty token")
	}
	return response.AccessToken
}

func getJSONTest(t *testing.T, url, token string, target any) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d", url, response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func postJSONTest(t *testing.T, url, token string, body any, target any, wantStatus int) {
	t.Helper()
	assertStatusTest(t, url, token, http.MethodPost, body, wantStatus, target)
}

func putJSONTest(t *testing.T, url, token string, body any, target any, wantStatus int) {
	t.Helper()
	assertStatusTest(t, url, token, http.MethodPut, body, wantStatus, target)
}

func patchJSONTest(t *testing.T, url, token string, body any, wantStatus int) {
	t.Helper()
	assertStatusTest(t, url, token, http.MethodPatch, body, wantStatus)
}

func assertStatusTest(t *testing.T, url, token, method string, body any, wantStatus int, targets ...any) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(method, url, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != wantStatus {
		t.Fatalf("%s %s returned %d, want %d: %s", method, url, response.StatusCode, wantStatus, string(responseBody))
	}
	if len(targets) > 0 {
		if err := json.Unmarshal(responseBody, targets[0]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSQLiteMigrationOpensExistingFile(t *testing.T) {
	databasePath := fmt.Sprintf("%s/shift.sqlite", t.TempDir())
	if err := os.WriteFile(databasePath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	// An empty file is a valid SQLite database for the migration layer.
	service, err := New(context.Background(), Config{DBType: "sqlite", DBDatabase: databasePath, JWTSecret: "test", SeedOnBoot: false})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
}

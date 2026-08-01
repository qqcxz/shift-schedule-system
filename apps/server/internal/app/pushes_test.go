package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestPushBindingAndStoreSettingsAPI(t *testing.T) {
	service, server, managerToken, staffToken := newPushTestServer(t)
	defer service.Close()
	defer server.Close()

	var binding pushBindingView
	getJSONTest(t, server.URL+"/api/push-settings/me", staffToken, &binding)
	if binding.TokenBound {
		t.Fatal("new staff account should not have a push token")
	}
	putJSONTest(t, server.URL+"/api/push-settings/me", staffToken, map[string]string{"xiatuiToken": "staff_token_123456"}, &binding, http.StatusOK)
	if !binding.TokenBound || binding.MaskedToken != "****3456" {
		t.Fatalf("unexpected binding view: %+v", binding)
	}

	var ciphertext string
	if err := service.db.QueryRow("SELECT `tokenCiphertext` FROM `push_bindings` LIMIT 1").Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if ciphertext == "staff_token_123456" {
		t.Fatal("push token must not be stored in plaintext")
	}

	assertStatusTest(t, server.URL+"/api/stores/current/push-settings", staffToken, http.MethodPatch, map[string]any{"swapRequestEnabled": true}, http.StatusForbidden)
	var settings storePushSettings
	assertStatusTest(t, server.URL+"/api/stores/current/push-settings", managerToken, http.MethodPatch, map[string]any{
		"schedulePublishEnabled":  true,
		"swapRequestEnabled":      true,
		"preShiftReminderEnabled": true,
		"preShiftReminderMinutes": 45,
		"timezone":                "Asia/Shanghai",
	}, http.StatusOK, &settings)
	if !settings.SchedulePublishEnabled || !settings.SwapRequestEnabled || settings.PreShiftReminderMinutes != 45 {
		t.Fatalf("unexpected store push settings: %+v", settings)
	}
	assertStatusTest(t, server.URL+"/api/push-settings/me/token", staffToken, http.MethodDelete, nil, http.StatusOK)
	getJSONTest(t, server.URL+"/api/push-settings/me", staffToken, &binding)
	if binding.TokenBound {
		t.Fatal("binding should be removed")
	}
}

func TestSwapAndPublishQueuePushDeliveries(t *testing.T) {
	service, server, managerToken, staffToken := newPushTestServer(t)
	defer service.Close()
	defer server.Close()

	var manager User
	getJSONTest(t, server.URL+"/api/auth/me", managerToken, &manager)
	var staff User
	getJSONTest(t, server.URL+"/api/auth/me", staffToken, &staff)
	managerCiphertext, err := service.encryptPushToken("manager_token_123456")
	if err != nil {
		t.Fatal(err)
	}
	staffCiphertext, err := service.encryptPushToken("staff_token_123456")
	if err != nil {
		t.Fatal(err)
	}
	now := nowDB()
	for _, row := range []struct {
		user       User
		ciphertext string
		token      string
	}{
		{manager, managerCiphertext, "manager_token_123456"},
		{staff, staffCiphertext, "staff_token_123456"},
	} {
		if _, err := service.db.Exec("INSERT INTO `push_bindings` (`userId`, `storeId`, `tokenCiphertext`, `tokenHash`, `tokenSuffix`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?)", row.user.ID, row.user.StoreID, row.ciphertext, hashPushToken(row.token), "3456", now, now); err != nil {
			t.Fatal(err)
		}
	}
	assertStatusTest(t, server.URL+"/api/stores/current/push-settings", managerToken, http.MethodPatch, map[string]any{"schedulePublishEnabled": true, "swapRequestEnabled": true}, http.StatusOK)

	var schedules struct {
		Month string         `json:"month"`
		Items []ScheduleItem `json:"items"`
	}
	getJSONTest(t, server.URL+"/api/schedules?month="+monthNow(), staffToken, &schedules)
	if len(schedules.Items) == 0 {
		t.Fatal("seed should contain schedules")
	}
	var target User
	var users []User
	getJSONTest(t, server.URL+"/api/users", staffToken, &users)
	for _, user := range users {
		if user.ID != staff.ID && user.IsActive {
			target = user
			break
		}
	}
	if target.ID == "" {
		t.Fatal("seed should contain a swap target")
	}
	var request ScheduleRequest
	postJSONTest(t, server.URL+"/api/requests", staffToken, map[string]any{
		"type":         RequestSwap,
		"fromDate":     schedules.Items[0].WorkDate,
		"targetUserId": target.ID,
		"reason":       "push integration test",
	}, &request, http.StatusCreated)

	postJSONTest(t, server.URL+"/api/schedules/"+schedules.Month+"/publish", managerToken, map[string]any{}, &map[string]any{}, http.StatusCreated)
	var swapCount, publishCount int
	if err := service.db.QueryRow("SELECT COUNT(*) FROM `push_deliveries` WHERE `eventType` = ?", pushEventSwapRequest).Scan(&swapCount); err != nil {
		t.Fatal(err)
	}
	if err := service.db.QueryRow("SELECT COUNT(*) FROM `push_deliveries` WHERE `eventType` = ?", pushEventSchedulePublish).Scan(&publishCount); err != nil {
		t.Fatal(err)
	}
	if swapCount != 1 || publishCount < 1 {
		t.Fatalf("unexpected queued deliveries: swap=%d publish=%d", swapCount, publishCount)
	}
}

func TestProviderParsingAndPersistentRateLimit(t *testing.T) {
	result := parseProviderResult(`{"code":200,"data":{"msg_id":"wx_20260623164956_8db1c691e77fe369","status":"processing"}}`)
	if result.MsgID != "wx_20260623164956_8db1c691e77fe369" || result.Status != "processing" {
		t.Fatalf("unexpected provider result: %+v", result)
	}

	service, server, _, _ := newPushTestServer(t)
	defer service.Close()
	defer server.Close()
	spec := rateLimitSpec{Scope: "test", Key: "same-key", Bucket: "same-bucket", Limit: 2}
	for index := 0; index < 2; index++ {
		ok, err := service.reserveRate(context.Background(), []rateLimitSpec{spec})
		if err != nil || !ok {
			t.Fatalf("reservation %d failed: ok=%v err=%v", index, ok, err)
		}
	}
	ok, err := service.reserveRate(context.Background(), []rateLimitSpec{spec})
	if err != nil || ok {
		t.Fatalf("third reservation should be denied: ok=%v err=%v", ok, err)
	}
}

func TestPersistentRateLimitIsAtomic(t *testing.T) {
	service, server, _, _ := newPushTestServer(t)
	defer service.Close()
	defer server.Close()

	const limit = 4
	const attempts = 24
	spec := rateLimitSpec{Scope: "concurrent-test", Key: "same-key", Bucket: "same-bucket", Limit: limit}
	type reservation struct {
		allowed bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan reservation, attempts)
	var workers sync.WaitGroup
	for range attempts {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			allowed, err := service.reserveRate(context.Background(), []rateLimitSpec{spec})
			results <- reservation{allowed: allowed, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	allowed := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.allowed {
			allowed++
		}
	}
	if allowed != limit {
		t.Fatalf("allowed %d reservations, want %d", allowed, limit)
	}
	var count int
	if err := service.db.QueryRow("SELECT `count` FROM `push_rate_buckets` WHERE `scope` = ? AND `bucketKey` = ? AND `bucketStart` = ?", spec.Scope, spec.Key, spec.Bucket).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != limit {
		t.Fatalf("stored count %d, want %d", count, limit)
	}
}

func TestRecoverStalePushDeliveries(t *testing.T) {
	service, server, managerToken, _ := newPushTestServer(t)
	defer service.Close()
	defer server.Close()

	var manager User
	getJSONTest(t, server.URL+"/api/auth/me", managerToken, &manager)
	stale := time.Now().UTC().Add(-pushDeliveryLease - time.Second).Format("2006-01-02 15:04:05.000")
	for _, delivery := range []struct {
		id    string
		state string
	}{
		{id: "stale-preparing", state: pushStatePreparing},
		{id: "stale-sending", state: pushStateSending},
	} {
		if _, err := service.db.Exec("INSERT INTO `push_deliveries` (`id`, `storeId`, `userId`, `eventType`, `dedupeKey`, `title`, `content`, `state`, `attempts`, `nextAttemptAt`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)", delivery.id, manager.StoreID, manager.ID, pushEventTest, delivery.id, "test", "test", delivery.state, stale, stale, stale); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.recoverStalePushDeliveries(context.Background()); err != nil {
		t.Fatal(err)
	}

	var preparingState, sendingState string
	var failedAttempts int
	if err := service.db.QueryRow("SELECT `state` FROM `push_deliveries` WHERE `id` = ?", "stale-preparing").Scan(&preparingState); err != nil {
		t.Fatal(err)
	}
	if err := service.db.QueryRow("SELECT `state`, `attempts` FROM `push_deliveries` WHERE `id` = ?", "stale-sending").Scan(&sendingState, &failedAttempts); err != nil {
		t.Fatal(err)
	}
	if preparingState != pushStatePending {
		t.Fatalf("stale preparing delivery state = %q, want %q", preparingState, pushStatePending)
	}
	if sendingState != pushStateFailed || failedAttempts != 1 {
		t.Fatalf("stale sending delivery state = %q, attempts = %d", sendingState, failedAttempts)
	}
}

func newPushTestServer(t *testing.T) (*Server, *httptest.Server, string, string) {
	t.Helper()
	databasePath := fmt.Sprintf("%s/shift.sqlite", t.TempDir())
	service, err := New(context.Background(), Config{
		DBType:             "sqlite",
		DBDatabase:         databasePath,
		JWTSecret:          "test-secret",
		PushEncryptionKey:  "test-push-encryption-key-32-bytes",
		PushEgressKey:      "test-egress",
		SeedOnBoot:         true,
		ScheduleKeepMonths: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler())
	managerToken := loginTestUserAfterSeed(t, server.URL, "qqcxz", "123456")
	staffToken := loginTestUserAfterSeed(t, server.URL, "staff1", "123456")
	return service, server, managerToken, staffToken
}

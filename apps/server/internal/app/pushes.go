package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	pushEventSchedulePublish = "schedule_publish"
	pushEventSwapRequest     = "swap_request"
	pushEventPreShift        = "pre_shift_reminder"
	pushEventTest            = "test"

	pushStatePending   = "pending"
	pushStatePreparing = "preparing"
	pushStateSending   = "sending"
	pushStateSent      = "sent"
	pushStateFailed    = "failed"
	pushStateRateLimit = "rate_limited"
)

var pushTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{6,256}$`)
var pushMsgIDPattern = regexp.MustCompile(`wx_[A-Za-z0-9_-]+`)
var pushTimePattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

type pushBindingView struct {
	Provider    string `json:"provider"`
	TokenBound  bool   `json:"tokenBound"`
	MaskedToken string `json:"maskedToken,omitempty"`
}

type pushBindingInput struct {
	XiatuiToken string `json:"xiatuiToken"`
}

type storePushSettings struct {
	SchedulePublishEnabled  bool   `json:"schedulePublishEnabled"`
	SwapRequestEnabled      bool   `json:"swapRequestEnabled"`
	PreShiftReminderEnabled bool   `json:"preShiftReminderEnabled"`
	PreShiftReminderTime    string `json:"preShiftReminderTime"`
	Timezone                string `json:"timezone"`
}

type storePushSettingsPatch struct {
	SchedulePublishEnabled  *bool   `json:"schedulePublishEnabled"`
	SwapRequestEnabled      *bool   `json:"swapRequestEnabled"`
	PreShiftReminderEnabled *bool   `json:"preShiftReminderEnabled"`
	PreShiftReminderTime    *string `json:"preShiftReminderTime"`
	Timezone                *string `json:"timezone"`
}

type pushDelivery struct {
	ID               string  `json:"id"`
	UserID           string  `json:"userId"`
	EventType        string  `json:"eventType"`
	RelatedID        *string `json:"relatedId,omitempty"`
	Title            string  `json:"title"`
	Content          string  `json:"content"`
	MsgID            *string `json:"msgId,omitempty"`
	State            string  `json:"state"`
	ProviderStatus   *string `json:"providerStatus,omitempty"`
	ProviderResponse *string `json:"providerResponse,omitempty"`
	Attempts         int     `json:"attempts"`
	AttemptedAt      *DBTime `json:"attemptedAt,omitempty"`
	StatusCheckedAt  *DBTime `json:"statusCheckedAt,omitempty"`
	CreatedAt        DBTime  `json:"createdAt"`
}

type providerResult struct {
	MsgID  string
	Status string
	Raw    string
}

func (s *Server) getPushBinding(w http.ResponseWriter, r *http.Request) error {
	view, err := s.pushBindingView(r.Context(), requestClaims(r).Subject)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, view)
	return nil
}

func (s *Server) putPushBinding(w http.ResponseWriter, r *http.Request) error {
	var input pushBindingInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	token := strings.TrimSpace(input.XiatuiToken)
	if !pushTokenPattern.MatchString(token) {
		return badRequest("Token 格式无效，请粘贴公众号提供的完整 Token")
	}
	claims := requestClaims(r)
	ciphertext, err := s.encryptPushToken(token)
	if err != nil {
		return err
	}
	hash := hashPushToken(token)
	suffix := token
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	now := nowDB()
	query := "INSERT INTO `push_bindings` (`userId`, `storeId`, `tokenCiphertext`, `tokenHash`, `tokenSuffix`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(`userId`) DO UPDATE SET `storeId` = excluded.`storeId`, `tokenCiphertext` = excluded.`tokenCiphertext`, `tokenHash` = excluded.`tokenHash`, `tokenSuffix` = excluded.`tokenSuffix`, `updatedAt` = excluded.`updatedAt`"
	if s.cfg.DBType == "mysql" {
		query = "INSERT INTO `push_bindings` (`userId`, `storeId`, `tokenCiphertext`, `tokenHash`, `tokenSuffix`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE `storeId` = VALUES(`storeId`), `tokenCiphertext` = VALUES(`tokenCiphertext`), `tokenHash` = VALUES(`tokenHash`), `tokenSuffix` = VALUES(`tokenSuffix`), `updatedAt` = VALUES(`updatedAt`)"
	}
	if _, err := s.db.ExecContext(r.Context(), query, claims.Subject, claims.StoreID, ciphertext, hash, suffix, now, now); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return badRequest("该 Token 已绑定到其他账号")
		}
		return err
	}
	writeJSON(w, http.StatusOK, pushBindingView{Provider: "虾推啥", TokenBound: true, MaskedToken: "****" + suffix})
	return nil
}

func (s *Server) deletePushBinding(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	if _, err := s.db.ExecContext(r.Context(), "DELETE FROM `push_bindings` WHERE `userId` = ? AND `storeId` = ?", claims.Subject, claims.StoreID); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (s *Server) getStorePushSettings(w http.ResponseWriter, r *http.Request) error {
	settings, err := s.loadStorePushSettings(r.Context(), s.db, requestClaims(r).StoreID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, settings)
	return nil
}

func (s *Server) patchStorePushSettings(w http.ResponseWriter, r *http.Request) error {
	var patch storePushSettingsPatch
	if err := decodeJSON(r, &patch); err != nil {
		return err
	}
	claims := requestClaims(r)
	settings, err := s.loadStorePushSettings(r.Context(), s.db, claims.StoreID)
	if err != nil {
		return err
	}
	if patch.SchedulePublishEnabled != nil {
		settings.SchedulePublishEnabled = *patch.SchedulePublishEnabled
	}
	if patch.SwapRequestEnabled != nil {
		settings.SwapRequestEnabled = *patch.SwapRequestEnabled
	}
	if patch.PreShiftReminderEnabled != nil {
		settings.PreShiftReminderEnabled = *patch.PreShiftReminderEnabled
	}
	if patch.PreShiftReminderTime != nil {
		settings.PreShiftReminderTime = strings.TrimSpace(*patch.PreShiftReminderTime)
	}
	if patch.Timezone != nil {
		settings.Timezone = strings.TrimSpace(*patch.Timezone)
	}
	if !pushTimePattern.MatchString(settings.PreShiftReminderTime) {
		return badRequest("班前提醒时间格式应为 HH:MM，例如 08:30")
	}
	if _, err := time.LoadLocation(settings.Timezone); err != nil {
		return badRequest("无效的时区")
	}
	now := nowDB()
	query := "INSERT INTO `store_push_settings` (`storeId`, `schedulePublishEnabled`, `swapRequestEnabled`, `preShiftReminderEnabled`, `preShiftReminderTime`, `timezone`, `updatedById`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(`storeId`) DO UPDATE SET `schedulePublishEnabled` = excluded.`schedulePublishEnabled`, `swapRequestEnabled` = excluded.`swapRequestEnabled`, `preShiftReminderEnabled` = excluded.`preShiftReminderEnabled`, `preShiftReminderTime` = excluded.`preShiftReminderTime`, `timezone` = excluded.`timezone`, `updatedById` = excluded.`updatedById`, `updatedAt` = excluded.`updatedAt`"
	if s.cfg.DBType == "mysql" {
		query = "INSERT INTO `store_push_settings` (`storeId`, `schedulePublishEnabled`, `swapRequestEnabled`, `preShiftReminderEnabled`, `preShiftReminderTime`, `timezone`, `updatedById`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE `schedulePublishEnabled` = VALUES(`schedulePublishEnabled`), `swapRequestEnabled` = VALUES(`swapRequestEnabled`), `preShiftReminderEnabled` = VALUES(`preShiftReminderEnabled`), `preShiftReminderTime` = VALUES(`preShiftReminderTime`), `timezone` = VALUES(`timezone`), `updatedById` = VALUES(`updatedById`), `updatedAt` = VALUES(`updatedAt`)"
	}
	if _, err := s.db.ExecContext(r.Context(), query, claims.StoreID, settings.SchedulePublishEnabled, settings.SwapRequestEnabled, settings.PreShiftReminderEnabled, settings.PreShiftReminderTime, settings.Timezone, claims.Subject, now, now); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, settings)
	return nil
}

func (s *Server) listPushDeliveries(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	query := "SELECT `id`, `userId`, `eventType`, `relatedId`, `title`, `content`, `msgId`, `state`, `providerStatus`, `providerResponse`, `attempts`, `attemptedAt`, `statusCheckedAt`, `createdAt` FROM `push_deliveries` WHERE `storeId` = ?"
	args := []any{claims.StoreID}
	if claims.Role != RoleManager {
		query += " AND `userId` = ?"
		args = append(args, claims.Subject)
	}
	query += " ORDER BY `createdAt` DESC LIMIT 100"
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]pushDelivery, 0)
	for rows.Next() {
		item, err := scanPushDelivery(rows)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
	return rows.Err()
}

func (s *Server) queryPushDeliveryStatus(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	item, err := s.findPushDelivery(r.Context(), r.PathValue("id"), claims)
	if err != nil {
		return err
	}
	if item.MsgID == nil || *item.MsgID == "" {
		return badRequest("该消息尚未取得 msg_id，暂不能查询")
	}
	if item.StatusCheckedAt != nil && time.Since(item.StatusCheckedAt.Time) < time.Minute && item.ProviderStatus != nil {
		writeJSON(w, http.StatusOK, item)
		return nil
	}
	if ok, err := s.reserveRate(r.Context(), []rateLimitSpec{{Scope: "status_ip_minute", Key: s.cfg.PushEgressKey, Bucket: time.Now().UTC().Format("2006-01-02T15:04"), Limit: 5}}); err != nil {
		return err
	} else if !ok {
		return &apiError{Status: http.StatusTooManyRequests, Message: "状态查询过于频繁，请稍后重试"}
	}
	result, err := s.xtuisStatus(r.Context(), *item.MsgID)
	if err != nil {
		return serviceUnavailable(err.Error())
	}
	now := nowDB()
	if _, err := s.db.ExecContext(r.Context(), "UPDATE `push_deliveries` SET `providerStatus` = ?, `providerResponse` = ?, `statusCheckedAt` = ?, `updatedAt` = ? WHERE `id` = ?", result.Status, result.Raw, now, now, item.ID); err != nil {
		return err
	}
	item, err = s.findPushDelivery(r.Context(), item.ID, claims)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, item)
	return nil
}

func (s *Server) pushBindingView(ctx context.Context, userID string) (pushBindingView, error) {
	var suffix string
	err := s.db.QueryRowContext(ctx, "SELECT `tokenSuffix` FROM `push_bindings` WHERE `userId` = ? LIMIT 1", userID).Scan(&suffix)
	if errors.Is(err, sql.ErrNoRows) {
		return pushBindingView{Provider: "虾推啥", TokenBound: false}, nil
	}
	if err != nil {
		return pushBindingView{}, err
	}
	return pushBindingView{Provider: "虾推啥", TokenBound: true, MaskedToken: "****" + suffix}, nil
}

func (s *Server) loadStorePushSettings(ctx context.Context, runner dbRunner, storeID string) (storePushSettings, error) {
	settings := storePushSettings{PreShiftReminderTime: "08:00", Timezone: "Asia/Shanghai"}
	err := runner.QueryRowContext(ctx, "SELECT `schedulePublishEnabled`, `swapRequestEnabled`, `preShiftReminderEnabled`, `preShiftReminderTime`, `timezone` FROM `store_push_settings` WHERE `storeId` = ? LIMIT 1", storeID).Scan(&settings.SchedulePublishEnabled, &settings.SwapRequestEnabled, &settings.PreShiftReminderEnabled, &settings.PreShiftReminderTime, &settings.Timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	return settings, err
}

func (s *Server) enqueuePush(ctx context.Context, runner dbRunner, storeID, userID, eventType, dedupeKey string, relatedID *string, title, content string) (string, error) {
	var exists int
	if err := runner.QueryRowContext(ctx, "SELECT COUNT(*) FROM `push_bindings` WHERE `userId` = ? AND `storeId` = ?", userID, storeID).Scan(&exists); err != nil {
		return "", err
	}
	if exists == 0 {
		return "", badRequest("尚未绑定虾推啥 Token")
	}
	id := uuid.NewString()
	now := nowDB()
	_, err := runner.ExecContext(ctx, "INSERT INTO `push_deliveries` (`id`, `storeId`, `userId`, `eventType`, `relatedId`, `dedupeKey`, `title`, `content`, `state`, `attempts`, `nextAttemptAt`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)", id, storeID, userID, eventType, nullableArg(relatedID), dedupeKey, truncateRunes(title, 255), truncateRunes(content, 65535), pushStatePending, now, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

func enqueuePushIfBound(ctx context.Context, runner dbRunner, storeID, userID, eventType, dedupeKey string, relatedID *string, title, content string) error {
	var exists int
	if err := runner.QueryRowContext(ctx, "SELECT COUNT(*) FROM `push_bindings` WHERE `userId` = ? AND `storeId` = ?", userID, storeID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	id := uuid.NewString()
	now := nowDB()
	_, err := runner.ExecContext(ctx, "INSERT INTO `push_deliveries` (`id`, `storeId`, `userId`, `eventType`, `relatedId`, `dedupeKey`, `title`, `content`, `state`, `attempts`, `nextAttemptAt`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)", id, storeID, userID, eventType, nullableArg(relatedID), dedupeKey, truncateRunes(title, 255), truncateRunes(content, 65535), pushStatePending, now, now, now)
	if err != nil && (strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate")) {
		return nil
	}
	return err
}

func (s *Server) enqueuePushToActiveManagers(ctx context.Context, runner dbRunner, storeID, eventType, relatedID, title, content string) error {
	rows, err := runner.QueryContext(ctx, "SELECT `id` FROM `users` WHERE `storeId` = ? AND `role` = ? AND `isActive` = 1", storeID, RoleManager)
	if err != nil {
		return err
	}
	defer rows.Close()
	managerIDs := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		managerIDs = append(managerIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range managerIDs {
		related := relatedID
		if err := enqueuePushIfBound(ctx, runner, storeID, id, eventType, eventType+":"+relatedID+":"+id, &related, title, content); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) encryptPushToken(token string) (string, error) {
	key := sha256.Sum256([]byte(s.cfg.PushEncryptionKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(token), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Server) decryptPushToken(encoded string) (string, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(s.cfg.PushEncryptionKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted push token")
	}
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
	return string(plain), err
}

func hashPushToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func (s *Server) xtuisSend(ctx context.Context, token, title, content string) (providerResult, error) {
	form := url.Values{}
	form.Set("text", title)
	form.Set("desp", content)
	endpoint := "https://wx.xtuis.cn/" + url.PathEscape(token) + ".send"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return providerResult{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.pushHTTPClient.Do(request)
	if err != nil {
		return providerResult{}, fmt.Errorf("虾推啥请求失败：%w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return providerResult{}, err
	}
	raw := strings.TrimSpace(string(body))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return providerResult{Raw: raw}, fmt.Errorf("虾推啥返回 HTTP %d", response.StatusCode)
	}
	result := parseProviderResult(raw)
	if result.MsgID == "" {
		return result, errors.New("虾推啥未返回 msg_id")
	}
	return result, nil
}

func (s *Server) xtuisStatus(ctx context.Context, msgID string) (providerResult, error) {
	if !pushMsgIDPattern.MatchString(msgID) {
		return providerResult{}, errors.New("无效的 msg_id")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://msgstatus.xtuis.cn/status/"+url.PathEscape(msgID), nil)
	if err != nil {
		return providerResult{}, err
	}
	response, err := s.pushHTTPClient.Do(request)
	if err != nil {
		return providerResult{}, fmt.Errorf("状态查询失败：%w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return providerResult{}, err
	}
	raw := strings.TrimSpace(string(body))
	if response.StatusCode == http.StatusTooManyRequests {
		return providerResult{Raw: raw}, &apiError{Status: http.StatusTooManyRequests, Message: "状态查询已触发虾推啥限流"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return providerResult{Raw: raw}, fmt.Errorf("状态接口返回 HTTP %d", response.StatusCode)
	}
	result := parseProviderResult(raw)
	result.MsgID = msgID
	return result, nil
}

func parseProviderResult(raw string) providerResult {
	result := providerResult{Raw: truncateRunes(raw, 4000), Status: "accepted"}
	if match := pushMsgIDPattern.FindString(raw); match != "" {
		result.MsgID = match
	}
	var payload any
	if json.Unmarshal([]byte(raw), &payload) == nil {
		findProviderFields(payload, &result)
	}
	return result
}

func findProviderFields(value any, result *providerResult) {
	switch value := value.(type) {
	case map[string]any:
		for key, nested := range value {
			lower := strings.ToLower(key)
			if text, ok := nested.(string); ok {
				if result.MsgID == "" && (lower == "msg_id" || lower == "msgid" || lower == "id") && pushMsgIDPattern.MatchString(text) {
					result.MsgID = text
				}
				if lower == "status" || lower == "state" || lower == "message" || lower == "msg" {
					result.Status = text
				}
			}
			findProviderFields(nested, result)
		}
	case []any:
		for _, nested := range value {
			findProviderFields(nested, result)
		}
	}
}

func (s *Server) findPushDelivery(ctx context.Context, id string, claims authClaims) (pushDelivery, error) {
	query := "SELECT `id`, `userId`, `eventType`, `relatedId`, `title`, `content`, `msgId`, `state`, `providerStatus`, `providerResponse`, `attempts`, `attemptedAt`, `statusCheckedAt`, `createdAt` FROM `push_deliveries` WHERE `id` = ? AND `storeId` = ?"
	args := []any{id, claims.StoreID}
	if claims.Role != RoleManager {
		query += " AND `userId` = ?"
		args = append(args, claims.Subject)
	}
	item, err := scanPushDelivery(s.db.QueryRowContext(ctx, query+" LIMIT 1", args...))
	if errors.Is(err, sql.ErrNoRows) {
		return pushDelivery{}, notFound("推送记录不存在")
	}
	return item, err
}

func scanPushDelivery(row scanner) (pushDelivery, error) {
	var item pushDelivery
	var relatedID, msgID, status, response sql.NullString
	var attemptedAt, statusCheckedAt sql.NullTime
	err := row.Scan(&item.ID, &item.UserID, &item.EventType, &relatedID, &item.Title, &item.Content, &msgID, &item.State, &status, &response, &item.Attempts, &attemptedAt, &statusCheckedAt, &item.CreatedAt)
	item.RelatedID = ptrFromNull(relatedID)
	item.MsgID = ptrFromNull(msgID)
	item.ProviderStatus = ptrFromNull(status)
	item.ProviderResponse = ptrFromNull(response)
	if attemptedAt.Valid {
		value := DBTime{Time: attemptedAt.Time.UTC()}
		item.AttemptedAt = &value
	}
	if statusCheckedAt.Valid {
		value := DBTime{Time: statusCheckedAt.Time.UTC()}
		item.StatusCheckedAt = &value
	}
	return item, err
}

type rateLimitSpec struct {
	Scope  string
	Key    string
	Bucket string
	Limit  int
}

func (s *Server) reserveRate(ctx context.Context, specs []rateLimitSpec) (bool, error) {
	if len(specs) == 0 {
		return true, nil
	}
	ordered := append([]rateLimitSpec(nil), specs...)
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].Scope != ordered[right].Scope {
			return ordered[left].Scope < ordered[right].Scope
		}
		if ordered[left].Key != ordered[right].Key {
			return ordered[left].Key < ordered[right].Key
		}
		return ordered[left].Bucket < ordered[right].Bucket
	})
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	for _, spec := range ordered {
		if spec.Limit < 1 {
			return false, fmt.Errorf("rate limit for %s must be positive", spec.Scope)
		}
		now := nowDB()
		query := "INSERT INTO `push_rate_buckets` (`scope`, `bucketKey`, `bucketStart`, `count`, `updatedAt`) VALUES (?, ?, ?, 1, ?) ON CONFLICT(`scope`, `bucketKey`, `bucketStart`) DO UPDATE SET `count` = `count` + 1, `updatedAt` = excluded.`updatedAt` WHERE `count` < ?"
		args := []any{spec.Scope, spec.Key, spec.Bucket, now, spec.Limit}
		if s.cfg.DBType == "mysql" {
			query = "INSERT INTO `push_rate_buckets` (`scope`, `bucketKey`, `bucketStart`, `count`, `updatedAt`) VALUES (?, ?, ?, 1, ?) ON DUPLICATE KEY UPDATE `updatedAt` = IF(`count` < ?, VALUES(`updatedAt`), `updatedAt`), `count` = IF(`count` < ?, `count` + 1, `count`)"
			args = []any{spec.Scope, spec.Key, spec.Bucket, now, spec.Limit, spec.Limit}
		}
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return false, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return false, err
		}
		if affected == 0 {
			return false, nil
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

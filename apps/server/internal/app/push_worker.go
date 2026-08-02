package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const pushDeliveryLease = 2 * time.Minute

type pushWorker struct {
	server *Server
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newPushHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (s *Server) startPushWorker(parent context.Context) {
	// The constructor context usually belongs to the database initialization request;
	// worker lifetime is owned by Server.Close instead of that short-lived context.
	_ = parent
	ctx, cancel := context.WithCancel(context.Background())
	worker := &pushWorker{server: s, cancel: cancel}
	worker.wg.Add(1)
	s.pushWorker = worker
	go worker.run(ctx)
}

func (w *pushWorker) close() {
	if w == nil {
		return
	}
	w.cancel()
	w.wg.Wait()
}

func (w *pushWorker) run(ctx context.Context) {
	defer w.wg.Done()
	interval := w.server.cfg.PushWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	reminderDone := make(chan struct{})
	go func() {
		defer close(reminderDone)
		w.runPreShiftReminderLoop(ctx)
	}()
	defer func() { <-reminderDone }()
	w.processPendingPushesTick(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processPendingPushesTick(ctx)
		}
	}
}

func (w *pushWorker) processPendingPushesTick(ctx context.Context) {
	if err := w.server.processPendingPushes(ctx, 20); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("process push deliveries: %v", err)
	}
}

func (w *pushWorker) runPreShiftReminderLoop(ctx context.Context) {
	w.enqueuePreShiftReminders(ctx)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.enqueuePreShiftReminders(ctx)
		}
	}
}

func (w *pushWorker) enqueuePreShiftReminders(ctx context.Context) {
	if err := w.server.enqueueDuePreShiftReminders(ctx, time.Now()); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("enqueue pre-shift reminders: %v", err)
	}
}

func (s *Server) processPendingPushes(ctx context.Context, limit int) error {
	if err := s.recoverStalePushDeliveries(ctx); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT `id` FROM `push_deliveries` WHERE `state` IN (?, ?) AND `nextAttemptAt` <= ? ORDER BY `createdAt` ASC LIMIT ?", pushStatePending, pushStateRateLimit, nowDB(), limit)
	if err != nil {
		return err
	}
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.processPushDelivery(ctx, id); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("process push delivery %s: %v", id, err)
		}
	}
	return nil
}

func (s *Server) processPushDelivery(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE `push_deliveries` SET `state` = ?, `updatedAt` = ? WHERE `id` = ? AND `state` IN (?, ?) AND `nextAttemptAt` <= ?", pushStatePreparing, nowDB(), id, pushStatePending, pushStateRateLimit, nowDB())
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return err
	}

	var tokenCiphertext, tokenHash, title, content string
	err = s.db.QueryRowContext(ctx, "SELECT b.`tokenCiphertext`, b.`tokenHash`, d.`title`, d.`content` FROM `push_deliveries` d JOIN `push_bindings` b ON b.`userId` = d.`userId` AND b.`storeId` = d.`storeId` WHERE d.`id` = ?", id).Scan(&tokenCiphertext, &tokenHash, &title, &content)
	if errors.Is(err, sql.ErrNoRows) {
		return s.failPushDelivery(ctx, id, "Token 已解绑，无法发送", "")
	}
	if err != nil {
		_ = s.requeuePushDelivery(ctx, id, "读取推送凭证失败，等待重试")
		return err
	}
	token, err := s.decryptPushToken(tokenCiphertext)
	if err != nil {
		return s.failPushDelivery(ctx, id, "Token 解密失败", err.Error())
	}
	now := time.Now().UTC()
	location, _ := time.LoadLocation("Asia/Shanghai")
	accountNow := now.In(location)
	allowed, err := s.reserveRate(ctx, []rateLimitSpec{
		{Scope: "send_token_minute", Key: tokenHash, Bucket: now.Format("2006-01-02T15:04"), Limit: 10},
		{Scope: "send_token_day", Key: tokenHash, Bucket: accountNow.Format("2006-01-02"), Limit: 300},
		{Scope: "send_ip_day", Key: s.cfg.PushEgressKey, Bucket: accountNow.Format("2006-01-02"), Limit: 5000},
	})
	if err != nil {
		_ = s.requeuePushDelivery(ctx, id, "限流服务暂不可用，等待重试")
		return err
	}
	if !allowed {
		next := now.Truncate(time.Minute).Add(time.Minute)
		_, err = s.db.ExecContext(ctx, "UPDATE `push_deliveries` SET `state` = ?, `providerStatus` = ?, `nextAttemptAt` = ?, `updatedAt` = ? WHERE `id` = ? AND `state` = ?", pushStateRateLimit, "本地限流等待", next.Format("2006-01-02 15:04:05.000"), nowDB(), id, pushStatePreparing)
		return err
	}

	result, err = s.db.ExecContext(ctx, "UPDATE `push_deliveries` SET `state` = ?, `updatedAt` = ? WHERE `id` = ? AND `state` = ?", pushStateSending, nowDB(), id, pushStatePreparing)
	if err != nil {
		return err
	}
	affected, err = result.RowsAffected()
	if err != nil || affected == 0 {
		return err
	}

	provider, sendErr := s.xtuisSend(ctx, token, title, content)
	attemptedAt := nowDB()
	if sendErr != nil {
		// A timeout can happen after the provider accepted the message. Do not
		// retry an ambiguous network failure automatically, or users may receive duplicates.
		return s.failPushDelivery(ctx, id, sendErr.Error(), provider.Raw)
	}
	writeCtx, cancel := pushDeliveryWriteContext(ctx)
	defer cancel()
	_, err = s.db.ExecContext(writeCtx, "UPDATE `push_deliveries` SET `tokenHash` = ?, `msgId` = ?, `state` = ?, `providerStatus` = ?, `providerResponse` = ?, `attempts` = `attempts` + 1, `attemptedAt` = ?, `updatedAt` = ? WHERE `id` = ?", tokenHash, provider.MsgID, pushStateSent, provider.Status, provider.Raw, attemptedAt, attemptedAt, id)
	return err
}

func (s *Server) failPushDelivery(ctx context.Context, id, status, response string) error {
	now := nowDB()
	writeCtx, cancel := pushDeliveryWriteContext(ctx)
	defer cancel()
	_, err := s.db.ExecContext(writeCtx, "UPDATE `push_deliveries` SET `state` = ?, `providerStatus` = ?, `providerResponse` = ?, `attempts` = `attempts` + 1, `attemptedAt` = ?, `updatedAt` = ? WHERE `id` = ?", pushStateFailed, truncateRunes(status, 255), truncateRunes(response, 4000), now, now, id)
	if err != nil {
		return err
	}
	return serviceUnavailable(status)
}

func (s *Server) recoverStalePushDeliveries(ctx context.Context) error {
	now := nowDB()
	cutoff := time.Now().UTC().Add(-pushDeliveryLease).Format("2006-01-02 15:04:05.000")
	if _, err := s.db.ExecContext(ctx, "UPDATE `push_deliveries` SET `state` = ?, `providerStatus` = ?, `nextAttemptAt` = ?, `updatedAt` = ? WHERE `state` = ? AND `updatedAt` < ?", pushStatePending, "发送准备超时，已重新排队", now, now, pushStatePreparing, cutoff); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE `push_deliveries` SET `state` = ?, `providerStatus` = ?, `attempts` = `attempts` + 1, `attemptedAt` = ?, `updatedAt` = ? WHERE `state` = ? AND `updatedAt` < ?", pushStateFailed, "发送过程已中断，无法确认是否已投递", now, now, pushStateSending, cutoff)
	return err
}

func (s *Server) requeuePushDelivery(ctx context.Context, id, status string) error {
	writeCtx, cancel := pushDeliveryWriteContext(ctx)
	defer cancel()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(writeCtx, "UPDATE `push_deliveries` SET `state` = ?, `providerStatus` = ?, `nextAttemptAt` = ?, `updatedAt` = ? WHERE `id` = ? AND `state` = ?", pushStatePending, truncateRunes(status, 255), now.Add(time.Minute).Format("2006-01-02 15:04:05.000"), now.Format("2006-01-02 15:04:05.000"), id, pushStatePreparing)
	return err
}

func pushDeliveryWriteContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent.Err() == nil {
		return parent, func() {}
	}
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func isRecordedPushFailure(err error) bool {
	var apiErr *apiError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusServiceUnavailable
}

func (s *Server) enqueueDuePreShiftReminders(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, "SELECT `storeId`, `preShiftReminderTime`, `timezone` FROM `store_push_settings` WHERE `preShiftReminderEnabled` = 1")
	if err != nil {
		return err
	}
	type setting struct {
		storeID    string
		remindTime string
		timezone   string
	}
	settings := make([]setting, 0)
	for rows.Next() {
		var item setting
		if err := rows.Scan(&item.storeID, &item.remindTime, &item.timezone); err != nil {
			rows.Close()
			return err
		}
		settings = append(settings, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range settings {
		location, err := time.LoadLocation(item.timezone)
		if err != nil {
			log.Printf("skip pre-shift reminder for store %s: invalid timezone %q", item.storeID, item.timezone)
			continue
		}
		localNow := now.In(location)
		if localNow.Format("15:04") != item.remindTime {
			continue
		}
		date := localNow.Format("2006-01-02")
		query := "SELECT sc.`id`, sc.`userId`, sc.`version`, u.`displayName`, st.`name`, st.`startTime`, st.`endTime` FROM `schedules` sc JOIN `users` u ON u.`id` = sc.`userId` AND u.`isActive` = 1 JOIN `shift_templates` st ON st.`id` = sc.`shiftTemplateId` AND st.`isOff` = 0 WHERE sc.`storeId` = ? AND sc.`workDate` = ? AND st.`startTime` > ?"
		upcoming, err := s.db.QueryContext(ctx, query, item.storeID, date, item.remindTime)
		if err != nil {
			return err
		}
		type reminder struct {
			scheduleID  string
			userID      string
			version     int
			displayName string
			shiftName   string
			startTime   string
			endTime     string
		}
		reminders := make([]reminder, 0)
		for upcoming.Next() {
			var reminder reminder
			if err := upcoming.Scan(&reminder.scheduleID, &reminder.userID, &reminder.version, &reminder.displayName, &reminder.shiftName, &reminder.startTime, &reminder.endTime); err != nil {
				upcoming.Close()
				return err
			}
			reminders = append(reminders, reminder)
		}
		if err := upcoming.Err(); err != nil {
			upcoming.Close()
			return err
		}
		if err := upcoming.Close(); err != nil {
			return err
		}
		for _, reminder := range reminders {
			related := reminder.scheduleID
			dedupe := fmt.Sprintf("pre-shift:%s:%d:%s", reminder.scheduleID, reminder.version, item.remindTime)
			title := fmt.Sprintf("%s 班前提醒", reminder.shiftName)
			content := fmt.Sprintf("%s，您今天 %s 上班。\n日期：%s\n班次：%s（%s-%s）", reminder.displayName, reminder.startTime, date, reminder.shiftName, reminder.startTime, reminder.endTime)
			if err := enqueuePushIfBound(ctx, s.db, item.storeID, reminder.userID, pushEventPreShift, dedupe, &related, title, content); err != nil {
				return err
			}
		}
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM `push_rate_buckets` WHERE `updatedAt` < ?", now.AddDate(0, 0, -2).UTC().Format("2006-01-02 15:04:05.000"))
	return nil
}

func pushPublishedScheduleMessage(userName, month string, items []publishedScheduleItem) (string, string) {
	title := fmt.Sprintf("%s 排班已发布", month)
	lines := []string{fmt.Sprintf("%s，您的排班已更新：", userName)}
	for _, item := range items {
		line := fmt.Sprintf("%s  %s", item.WorkDate, item.ShiftName)
		if item.StartTime != "" && item.EndTime != "" && !item.IsOff {
			line += fmt.Sprintf("（%s-%s）", item.StartTime, item.EndTime)
		}
		lines = append(lines, line)
	}
	if len(items) >= 20 {
		lines = append(lines, "仅展示本次发布的前 20 项，请登录排班系统查看完整内容。")
	}
	return title, strings.Join(lines, "\n")
}

type publishedScheduleItem struct {
	WorkDate  string
	ShiftName string
	StartTime string
	EndTime   string
	IsOff     bool
}

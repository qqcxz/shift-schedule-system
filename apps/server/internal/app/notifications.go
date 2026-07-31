package app

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func createNotification(ctx context.Context, runner dbRunner, storeID, userID, title, content string, relatedType, relatedID *string) (Notification, error) {
	var recipientCount int
	if err := runner.QueryRowContext(ctx, "SELECT COUNT(*) FROM `users` WHERE `id` = ? AND `storeId` = ?", userID, storeID).Scan(&recipientCount); err != nil {
		return Notification{}, err
	}
	if recipientCount == 0 {
		return Notification{}, badRequest("通知接收人不存在")
	}
	id := uuid.NewString()
	now := nowDB()
	_, err := runner.ExecContext(ctx, "INSERT INTO `notifications` (`id`, `storeId`, `userId`, `title`, `content`, `isRead`, `relatedType`, `relatedId`, `createdAt`) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)", id, storeID, userID, title, content, nullableArg(relatedType), nullableArg(relatedID), now)
	if err != nil {
		return Notification{}, err
	}
	item, err := scanNotification(runner.QueryRowContext(ctx, "SELECT "+notificationColumns+" FROM `notifications` WHERE `id` = ? LIMIT 1", id))
	if err != nil {
		return Notification{}, err
	}
	if err := trimUserInbox(ctx, runner, userID, 100); err != nil {
		return Notification{}, err
	}
	return item, nil
}

func nullableArg(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func notifyManagers(ctx context.Context, runner dbRunner, storeID, title, content string, relatedType, relatedID *string) ([]Notification, error) {
	rows, err := runner.QueryContext(ctx, "SELECT `id` FROM `users` WHERE `storeId` = ? AND `role` = ? AND `isActive` = ?", storeID, RoleManager, true)
	if err != nil {
		return nil, err
	}
	managerIDs := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		managerIDs = append(managerIDs, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]Notification, 0, len(managerIDs))
	for _, managerID := range managerIDs {
		item, err := createNotification(ctx, runner, storeID, managerID, title, content, relatedType, relatedID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func trimUserInbox(ctx context.Context, runner dbRunner, userID string, keep int) error {
	rows, err := runner.QueryContext(ctx, "SELECT `id` FROM `notifications` WHERE `userId` = ? ORDER BY `createdAt` DESC LIMIT 500 OFFSET ?", userID, keep)
	if err != nil {
		return err
	}
	overflow := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		overflow = append(overflow, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(overflow) == 0 {
		return nil
	}
	for _, id := range overflow {
		if _, err := runner.ExecContext(ctx, "DELETE FROM `notifications` WHERE `id` = ?", id); err != nil {
			return err
		}
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02 15:04:05.000")
	_, err = runner.ExecContext(ctx, "DELETE FROM `notifications` WHERE `userId` = ? AND `isRead` = 1 AND `createdAt` < ?", userID, cutoff)
	return err
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.QueryContext(r.Context(), "SELECT "+notificationColumns+" FROM `notifications` WHERE `userId` = ? ORDER BY `createdAt` DESC LIMIT 100", requestClaims(r).Subject)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, items)
	return nil
}

func (s *Server) markNotificationRead(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	result, err := s.db.ExecContext(r.Context(), "UPDATE `notifications` SET `isRead` = 1 WHERE `id` = ? AND `userId` = ?", r.PathValue("id"), claims.Subject)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		writeJSON(w, http.StatusCreated, nil)
		return nil
	}
	item, err := scanNotification(s.db.QueryRowContext(r.Context(), "SELECT "+notificationColumns+" FROM `notifications` WHERE `id` = ? AND `userId` = ?", r.PathValue("id"), claims.Subject))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, item)
	return nil
}

func (s *Server) markAllNotificationsRead(w http.ResponseWriter, r *http.Request) error {
	_, err := s.db.ExecContext(r.Context(), "UPDATE `notifications` SET `isRead` = 1 WHERE `userId` = ? AND `isRead` = 0", requestClaims(r).Subject)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
	return nil
}

type clearNotificationsInput struct {
	Mode string `json:"mode"`
}

func (s *Server) clearNotifications(w http.ResponseWriter, r *http.Request) error {
	var input clearNotificationsInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	mode := "read"
	query := "DELETE FROM `notifications` WHERE `userId` = ? AND `isRead` = 1"
	if input.Mode == "all" {
		mode = "all"
		query = "DELETE FROM `notifications` WHERE `userId` = ?"
	}
	deleted, err := execAffected(r.Context(), s.db, query, requestClaims(r).Subject)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "deleted": deleted, "mode": mode})
	return nil
}

func (s *Server) deleteNotification(w http.ResponseWriter, r *http.Request) error {
	deleted, err := execAffected(r.Context(), s.db, "DELETE FROM `notifications` WHERE `id` = ? AND `userId` = ?", r.PathValue("id"), requestClaims(r).Subject)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": deleted})
	return nil
}

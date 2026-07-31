package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_\x{4e00}-\x{9fa5}.-]{2,32}$`)

type createUserInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type updateUserInput struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"displayName"`
	Password    *string `json:"password"`
	Role        *string `json:"role"`
	IsActive    *bool   `json:"isActive"`
}

func validRole(role string) bool {
	return role == RoleManager || role == RoleStaff
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return badRequest("用户名仅支持字母、数字、下划线、中文、. -")
	}
	return nil
}

func uniqueUsername(ctx context.Context, runner dbRunner, username, excludeID string) error {
	query := "SELECT COUNT(*) FROM `users` WHERE `username` = ?"
	args := []any{username}
	if excludeID != "" {
		query += " AND `id` <> ?"
		args = append(args, excludeID)
	}
	var count int
	if err := runner.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return badRequest("用户名已存在")
	}
	return nil
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	includeInactive := claims.Role == RoleManager && (r.URL.Query().Get("includeInactive") == "1" || r.URL.Query().Get("includeInactive") == "true")
	query := "SELECT " + userColumns + " FROM `users` WHERE `storeId` = ?"
	args := []any{claims.StoreID}
	if !includeInactive {
		query += " AND `isActive` = ?"
		args = append(args, true)
	}
	query += " ORDER BY `role` ASC, `displayName` ASC"
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, users)
	return nil
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) error {
	var input createUserInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	username := normalizeUsername(input.Username)
	if err := validateUsername(username); err != nil {
		return err
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if length := len([]rune(displayName)); length < 2 || length > 32 {
		return badRequest("员工姓名长度应为 2 到 32 个字符")
	}
	if length := len([]rune(input.Password)); length < 6 || length > 64 {
		return badRequest("密码长度应为 6 到 64 个字符")
	}
	role := input.Role
	if role == "" {
		role = RoleStaff
	}
	if !validRole(role) {
		return badRequest("角色无效")
	}
	if err := uniqueUsername(r.Context(), s.db, username, ""); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
	if err != nil {
		return err
	}
	claims := requestClaims(r)
	user := User{
		ID:           uuid.NewString(),
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		Role:         role,
		StoreID:      claims.StoreID,
		IsActive:     true,
	}
	now := nowDB()
	_, err = s.db.ExecContext(r.Context(), "INSERT INTO `users` (`id`, `username`, `displayName`, `passwordHash`, `role`, `storeId`, `isActive`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", user.ID, user.Username, user.DisplayName, user.PasswordHash, user.Role, user.StoreID, user.IsActive, now, now)
	if err != nil {
		return err
	}
	s.realtime.EmitToStore(claims.StoreID, "users.updated", map[string]any{"action": "created", "userId": user.ID})
	writeJSON(w, http.StatusCreated, user)
	return nil
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) error {
	var input updateUserInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	if input.Username != nil {
		length := len([]rune(*input.Username))
		if length < 2 || length > 32 {
			return badRequest("用户名长度应为 2 到 32 个字符")
		}
	}
	if input.DisplayName != nil {
		displayName := strings.TrimSpace(*input.DisplayName)
		length := len([]rune(displayName))
		if length < 2 || length > 32 {
			return badRequest("员工姓名长度应为 2 到 32 个字符")
		}
	}
	if input.Password != nil && *input.Password != "" {
		length := len([]rune(*input.Password))
		if length < 6 || length > 64 {
			return badRequest("密码长度应为 6 到 64 个字符")
		}
	}
	if input.Role != nil && !validRole(*input.Role) {
		return badRequest("角色无效")
	}

	claims := requestClaims(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	user, err := findUserInStore(r.Context(), tx, claims.StoreID, r.PathValue("id"))
	if err != nil {
		return err
	}
	oldPasswordHash := user.PasswordHash
	oldRole := user.Role
	oldActive := user.IsActive
	if input.DisplayName != nil {
		user.DisplayName = strings.TrimSpace(*input.DisplayName)
	}
	if input.Username != nil {
		username := normalizeUsername(*input.Username)
		if err := validateUsername(username); err != nil {
			return err
		}
		if err := uniqueUsername(r.Context(), tx, username, user.ID); err != nil {
			return err
		}
		user.Username = username
	}
	if input.Password != nil && *input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), 10)
		if err != nil {
			return err
		}
		user.PasswordHash = string(hash)
	}
	if input.Role != nil && *input.Role != user.Role {
		if user.ID == claims.Subject && *input.Role != RoleManager {
			return badRequest("不能取消自己的店长权限")
		}
		user.Role = *input.Role
	}
	if input.IsActive != nil && *input.IsActive != user.IsActive {
		if !*input.IsActive {
			if user.ID == claims.Subject {
				return badRequest("不能停用自己的账号")
			}
		}
		user.IsActive = *input.IsActive
	}
	if oldRole == RoleManager && oldActive && (user.Role != RoleManager || !user.IsActive) {
		count, err := s.activeManagerCountForUpdate(r.Context(), tx, claims.StoreID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return badRequest("门店至少保留一名店长")
		}
	}

	query := "UPDATE `users` SET `username` = ?, `displayName` = ?, `passwordHash` = ?, `role` = ?, `isActive` = ?, `updatedAt` = ?"
	args := []any{user.Username, user.DisplayName, user.PasswordHash, user.Role, user.IsActive, nowDB()}
	if oldPasswordHash != user.PasswordHash {
		query += ", `authVersion` = `authVersion` + 1"
	}
	query += " WHERE `id` = ? AND `storeId` = ?"
	args = append(args, user.ID, claims.StoreID)
	_, err = tx.ExecContext(r.Context(), query, args...)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if oldPasswordHash != user.PasswordHash || oldRole != user.Role || oldActive != user.IsActive || !user.IsActive {
		s.realtime.DisconnectUser(user.ID)
	}
	s.realtime.EmitToStore(claims.StoreID, "users.updated", map[string]any{"action": "updated", "userId": user.ID})
	writeJSON(w, http.StatusOK, user)
	return nil
}

func findUserInStore(ctx context.Context, runner dbRunner, storeID, id string) (User, error) {
	user, err := scanUser(runner.QueryRowContext(ctx, "SELECT "+userColumns+" FROM `users` WHERE `id` = ? AND `storeId` = ? LIMIT 1", id, storeID))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, notFound("用户不存在")
	}
	return user, err
}

func (s *Server) activeManagerCountForUpdate(ctx context.Context, runner dbRunner, storeID string) (int, error) {
	query := "SELECT `id` FROM `users` WHERE `storeId` = ? AND `role` = ? AND `isActive` = ?"
	if s.cfg.DBType == "mysql" {
		query += " FOR UPDATE"
	}
	rows, err := runner.QueryContext(ctx, query, storeID, RoleManager, true)
	if err != nil {
		return 0, err
	}
	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		count++
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	user, err := findUserInStore(r.Context(), tx, claims.StoreID, r.PathValue("id"))
	if err != nil {
		return err
	}
	if user.ID == claims.Subject {
		return badRequest("不能删除自己的账号")
	}
	if user.Role == RoleManager && user.IsActive {
		count, err := s.activeManagerCountForUpdate(r.Context(), tx, claims.StoreID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return badRequest("门店至少保留一名店长")
		}
	}

	requestRows, err := tx.QueryContext(r.Context(), "SELECT `id` FROM `schedule_requests` WHERE `storeId` = ? AND (`requesterId` = ? OR `targetUserId` = ?)", claims.StoreID, user.ID, user.ID)
	if err != nil {
		return err
	}
	requestIDs := make([]string, 0)
	for requestRows.Next() {
		var id string
		if err := requestRows.Scan(&id); err != nil {
			requestRows.Close()
			return err
		}
		requestIDs = append(requestIDs, id)
	}
	if err := requestRows.Close(); err != nil {
		return err
	}
	if err := requestRows.Err(); err != nil {
		return err
	}

	deletedSchedules, err := execAffected(r.Context(), tx, "DELETE FROM `schedules` WHERE `storeId` = ? AND `userId` = ?", claims.StoreID, user.ID)
	if err != nil {
		return err
	}
	deletedNotifications, err := execAffected(r.Context(), tx, "DELETE FROM `notifications` WHERE `storeId` = ? AND `userId` = ?", claims.StoreID, user.ID)
	if err != nil {
		return err
	}
	deletedRequests := int64(0)
	for _, requestID := range requestIDs {
		count, err := execAffected(r.Context(), tx, "DELETE FROM `notifications` WHERE `storeId` = ? AND `relatedType` = 'request' AND `relatedId` = ?", claims.StoreID, requestID)
		if err != nil {
			return err
		}
		deletedNotifications += count
		count, err = execAffected(r.Context(), tx, "DELETE FROM `schedule_requests` WHERE `storeId` = ? AND `id` = ?", claims.StoreID, requestID)
		if err != nil {
			return err
		}
		deletedRequests += count
	}
	result, err := tx.ExecContext(r.Context(), "DELETE FROM `users` WHERE `id` = ? AND `storeId` = ?", user.ID, claims.StoreID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return notFound("用户不存在")
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	s.realtime.DisconnectUser(user.ID)
	s.realtime.EmitToStore(claims.StoreID, "users.updated", map[string]any{"action": "deleted", "userId": user.ID})
	if deletedSchedules > 0 {
		s.realtime.EmitToStore(claims.StoreID, "schedule.updated", map[string]any{"action": "user-deleted", "storeId": claims.StoreID, "userId": user.ID, "deleted": deletedSchedules})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                   true,
		"deletedSchedules":     deletedSchedules,
		"deletedRequests":      deletedRequests,
		"deletedNotifications": deletedNotifications,
	})
	return nil
}

func execAffected(ctx context.Context, runner dbRunner, query string, args ...any) (int64, error) {
	result, err := runner.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

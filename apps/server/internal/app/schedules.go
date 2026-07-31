package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var (
	monthPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)
	datePattern  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

const maxScheduleItems = 10000

type saveMonthInput struct {
	Month string                   `json:"month"`
	Items []saveMonthScheduleInput `json:"items"`
}

type saveMonthScheduleInput struct {
	UserID          string  `json:"userId"`
	WorkDate        string  `json:"workDate"`
	ShiftTemplateID string  `json:"shiftTemplateId"`
	Status          *string `json:"status"`
	Note            *string `json:"note"`
	Version         *int    `json:"version"`
}

type cleanupSchedulesInput struct {
	BeforeMonth *string `json:"beforeMonth"`
	KeepMonths  *int    `json:"keepMonths"`
}

func monthRange(month string) (string, string, error) {
	if !monthPattern.MatchString(month) || month[:4] == "0000" {
		return "", "", badRequest("month 格式应为 YYYY-MM")
	}
	first, err := time.ParseInLocation("2006-01", month, time.UTC)
	if err != nil {
		return "", "", badRequest("month 格式应为 YYYY-MM")
	}
	last := first.AddDate(0, 1, -1)
	return month + "-01", fmt.Sprintf("%s-%02d", month, last.Day()), nil
}

func shiftMonth(month string, offset int) string {
	first, _ := time.ParseInLocation("2006-01", month, time.UTC)
	shifted := first.AddDate(0, offset, 0)
	return fmt.Sprintf("%04d-%02d", shifted.Year(), int(shifted.Month()))
}

func parseISODate(value string) (time.Time, error) {
	if !datePattern.MatchString(value) || value[:4] == "0000" {
		return time.Time{}, badRequest("日期格式应为 YYYY-MM-DD")
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, badRequest("日期格式应为 YYYY-MM-DD")
	}
	return parsed, nil
}

func validateSaveMonth(input saveMonthInput) error {
	start, end, err := monthRange(input.Month)
	if err != nil {
		return err
	}
	if len(input.Items) == 0 {
		return badRequest("items 至少需要一项")
	}
	if len(input.Items) > maxScheduleItems {
		return badRequest(fmt.Sprintf("items 不能超过 %d 项", maxScheduleItems))
	}
	seen := make(map[string]struct{}, len(input.Items))
	for _, item := range input.Items {
		if item.UserID == "" || item.ShiftTemplateID == "" {
			return badRequest("排班数据格式错误")
		}
		if _, err := parseISODate(item.WorkDate); err != nil {
			return err
		}
		if item.WorkDate < start || item.WorkDate > end {
			return badRequest("排班日期必须属于指定月份")
		}
		if item.Status != nil && len([]rune(*item.Status)) > 32 {
			return badRequest("排班状态不能超过 32 个字符")
		}
		if item.Note != nil && len([]rune(*item.Note)) > 255 {
			return badRequest("排班备注不能超过 255 个字符")
		}
		key := item.UserID + "\x00" + item.WorkDate
		if _, exists := seen[key]; exists {
			return badRequest("同一员工同一天不能重复提交排班")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (s *Server) getMonthSchedules(w http.ResponseWriter, r *http.Request) error {
	result, err := s.monthSchedules(r.Context(), requestClaims(r).StoreID, r.URL.Query().Get("month"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}

func (s *Server) monthSchedules(ctx context.Context, storeID, month string) (map[string]any, error) {
	start, end, err := monthRange(month)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT s.`+"`id`"+`, s.`+"`userId`"+`, u.`+"`displayName`"+`, s.`+"`workDate`"+`, s.`+"`shiftTemplateId`"+`,
       sh.`+"`name`"+`, sh.`+"`color`"+`, s.`+"`status`"+`, s.`+"`version`"+`, s.`+"`note`"+`
FROM `+"`schedules`"+` s
INNER JOIN `+"`users`"+` u ON u.`+"`id`"+` = s.`+"`userId`"+` AND u.`+"`storeId`"+` = s.`+"`storeId`"+`
INNER JOIN `+"`shift_templates`"+` sh ON sh.`+"`id`"+` = s.`+"`shiftTemplateId`"+` AND sh.`+"`storeId`"+` = s.`+"`storeId`"+`
WHERE s.`+"`storeId`"+` = ? AND s.`+"`workDate`"+` BETWEEN ? AND ?
ORDER BY s.`+"`workDate`"+` ASC`, storeID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ScheduleItem, 0)
	for rows.Next() {
		var item ScheduleItem
		var workDate DBDate
		var note sql.NullString
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserName, &workDate, &item.ShiftTemplateID, &item.ShiftName, &item.ShiftColor, &item.Status, &item.Version, &note); err != nil {
			return nil, err
		}
		item.WorkDate = string(workDate)
		item.Note = ptrFromNull(note)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"month": month, "items": items}, nil
}

func (s *Server) scheduleStorage(w http.ResponseWriter, r *http.Request) error {
	result, err := s.storageSummary(r.Context(), requestClaims(r).StoreID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}

func (s *Server) storageSummary(ctx context.Context, storeID string) (map[string]any, error) {
	var total int64
	var oldest, newest sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), MIN(`workDate`), MAX(`workDate`) FROM `schedules` WHERE `storeId` = ?", storeID).Scan(&total, &oldest, &newest)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"total":             total,
		"oldestDate":        nullableValue(oldest),
		"newestDate":        nullableValue(newest),
		"keepMonthsDefault": s.cfg.ScheduleKeepMonths,
	}, nil
}

func nullableValue(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func (s *Server) cleanupSchedules(w http.ResponseWriter, r *http.Request) error {
	var input cleanupSchedulesInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	if input.BeforeMonth == nil && input.KeepMonths == nil {
		return badRequest("请指定 beforeMonth 或 keepMonths")
	}
	cutoffMonth := ""
	if input.BeforeMonth != nil {
		cutoffMonth = *input.BeforeMonth
		if _, _, err := monthRange(cutoffMonth); err != nil {
			return badRequest("beforeMonth 格式应为 YYYY-MM")
		}
	}
	if input.KeepMonths != nil {
		if *input.KeepMonths < 1 || *input.KeepMonths > 120 {
			return badRequest("keepMonths 应为 1 到 120")
		}
		keepCutoff := shiftMonth(monthNow(), -(*input.KeepMonths - 1))
		if cutoffMonth == "" || keepCutoff < cutoffMonth {
			cutoffMonth = keepCutoff
		}
	}
	if cutoffMonth == "" {
		return badRequest("beforeMonth 格式应为 YYYY-MM")
	}
	if _, _, err := monthRange(cutoffMonth); err != nil {
		return badRequest("beforeMonth 格式应为 YYYY-MM")
	}
	claims := requestClaims(r)
	deleted, err := execAffected(r.Context(), s.db, "DELETE FROM `schedules` WHERE `storeId` = ? AND `workDate` < ?", claims.StoreID, cutoffMonth+"-01")
	if err != nil {
		return err
	}
	s.realtime.EmitToStore(claims.StoreID, "schedule.updated", map[string]any{"action": "cleanup", "storeId": claims.StoreID, "beforeMonth": cutoffMonth, "deleted": deleted})
	summary, err := s.storageSummary(r.Context(), claims.StoreID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "beforeMonth": cutoffMonth, "deleted": deleted, "summary": summary})
	return nil
}

func (s *Server) saveMonthSchedules(w http.ResponseWriter, r *http.Request) error {
	var input saveMonthInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	if err := validateSaveMonth(input); err != nil {
		return err
	}
	claims := requestClaims(r)
	shiftIDs := make(map[string]bool)
	for _, item := range input.Items {
		shiftIDs[item.ShiftTemplateID] = true
	}
	for shiftID := range shiftIDs {
		var count int
		if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM `shift_templates` WHERE `id` = ? AND `storeId` = ?", shiftID, claims.StoreID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return badRequest("存在无效班次")
		}
	}
	userIDs := make(map[string]struct{}, len(input.Items))
	for _, item := range input.Items {
		userIDs[item.UserID] = struct{}{}
	}
	for userID := range userIDs {
		user, err := findUserInStore(r.Context(), s.db, claims.StoreID, userID)
		if err != nil {
			if isNotFoundError(err) {
				return badRequest("存在无效员工")
			}
			return err
		}
		if !user.IsActive {
			return badRequest("不能为已停用员工排班")
		}
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range input.Items {
		existing, err := findSchedule(r.Context(), tx, claims.StoreID, item.UserID, item.WorkDate)
		if errors.Is(err, sql.ErrNoRows) {
			status := "normal"
			if item.Status != nil && *item.Status != "" {
				status = *item.Status
			}
			var note any
			if item.Note != nil {
				note = *item.Note
			}
			now := nowDB()
			_, err = tx.ExecContext(r.Context(), "INSERT INTO `schedules` (`id`, `storeId`, `userId`, `workDate`, `shiftTemplateId`, `status`, `version`, `note`, `createdById`, `updatedById`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?)", uuid.NewString(), claims.StoreID, item.UserID, item.WorkDate, item.ShiftTemplateID, status, note, claims.Subject, claims.Subject, now, now)
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if item.Version != nil && *item.Version != 0 && *item.Version != existing.Version {
			return badRequest(fmt.Sprintf("排班已被其他人更新：%s / %s", item.WorkDate, item.UserID))
		}
		status := existing.Status
		if item.Status != nil && *item.Status != "" {
			status = *item.Status
		}
		note := any(nil)
		if existing.Note.Valid {
			note = existing.Note.String
		}
		if item.Note != nil {
			note = *item.Note
		}
		query := "UPDATE `schedules` SET `shiftTemplateId` = ?, `status` = ?, `note` = ?, `updatedById` = ?, `version` = ?, `updatedAt` = ? WHERE `id` = ?"
		args := []any{item.ShiftTemplateID, status, note, claims.Subject, existing.Version + 1, nowDB(), existing.ID}
		if item.Version != nil && *item.Version != 0 {
			query += " AND `version` = ?"
			args = append(args, existing.Version)
		}
		result, updateErr := tx.ExecContext(r.Context(), query, args...)
		if updateErr != nil {
			return updateErr
		}
		updated, updateErr := result.RowsAffected()
		if updateErr != nil {
			return updateErr
		}
		if updated == 0 {
			return badRequest(fmt.Sprintf("排班已被其他人更新：%s / %s", item.WorkDate, item.UserID))
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.realtime.EmitToStore(claims.StoreID, "schedule.updated", map[string]any{"month": input.Month, "storeId": claims.StoreID, "count": len(input.Items)})
	result, err := s.monthSchedules(r.Context(), claims.StoreID, input.Month)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}

func applyLeave(ctx context.Context, runner dbRunner, storeID, userID, workDate, offShiftID, operatorID string) error {
	existing, err := findSchedule(ctx, runner, storeID, userID, workDate)
	if errors.Is(err, sql.ErrNoRows) {
		now := nowDB()
		_, err = runner.ExecContext(ctx, "INSERT INTO `schedules` (`id`, `storeId`, `userId`, `workDate`, `shiftTemplateId`, `status`, `version`, `createdById`, `updatedById`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, 'leave', 1, ?, ?, ?, ?)", uuid.NewString(), storeID, userID, workDate, offShiftID, operatorID, operatorID, now, now)
		return err
	}
	if err != nil {
		return err
	}
	return updateScheduleVersion(ctx, runner, existing, offShiftID, "leave", operatorID)
}

func applySwap(ctx context.Context, runner dbRunner, storeID, requesterID, targetUserID, workDate, operatorID string) error {
	if requesterID == targetUserID {
		return badRequest("调班目标不能是自己")
	}
	requester, err := findSchedule(ctx, runner, storeID, requesterID, workDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("调班双方当天排班不完整")
		}
		return err
	}
	target, err := findSchedule(ctx, runner, storeID, targetUserID, workDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("调班双方当天排班不完整")
		}
		return err
	}
	if err := updateScheduleVersion(ctx, runner, requester, target.ShiftTemplateID, "swapped", operatorID); err != nil {
		return err
	}
	return updateScheduleVersion(ctx, runner, target, requester.ShiftTemplateID, "swapped", operatorID)
}

func applyChange(ctx context.Context, runner dbRunner, storeID, userID, workDate, toShiftID, operatorID string) error {
	if _, err := findShift(ctx, runner, storeID, toShiftID); err != nil {
		if isNotFoundError(err) {
			return badRequest("目标班次不存在")
		}
		return err
	}
	existing, err := findSchedule(ctx, runner, storeID, userID, workDate)
	if errors.Is(err, sql.ErrNoRows) {
		now := nowDB()
		_, err = runner.ExecContext(ctx, "INSERT INTO `schedules` (`id`, `storeId`, `userId`, `workDate`, `shiftTemplateId`, `status`, `version`, `createdById`, `updatedById`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, 'normal', 1, ?, ?, ?, ?)", uuid.NewString(), storeID, userID, workDate, toShiftID, operatorID, operatorID, now, now)
		return err
	}
	if err != nil {
		return err
	}
	return updateScheduleVersion(ctx, runner, existing, toShiftID, "normal", operatorID)
}

func updateScheduleVersion(ctx context.Context, runner dbRunner, existing Schedule, shiftID, status, operatorID string) error {
	result, err := runner.ExecContext(ctx, "UPDATE `schedules` SET `shiftTemplateId` = ?, `status` = ?, `updatedById` = ?, `version` = ?, `updatedAt` = ? WHERE `id` = ? AND `version` = ?", shiftID, status, operatorID, existing.Version+1, nowDB(), existing.ID, existing.Version)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return badRequest("排班已被其他人更新")
	}
	return nil
}

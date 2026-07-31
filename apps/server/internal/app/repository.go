package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type scanner interface {
	Scan(dest ...any) error
}

type dbRunner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const userColumns = "`id`, `username`, `displayName`, `passwordHash`, `role`, `storeId`, `isActive`, `authVersion`, `mustChangePassword`, `createdAt`, `updatedAt`"

func scanUser(row scanner) (User, error) {
	var user User
	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Role,
		&user.StoreID,
		&user.IsActive,
		&user.AuthVersion,
		&user.MustChangePassword,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func findUserByID(ctx context.Context, runner dbRunner, id string) (User, error) {
	user, err := scanUser(runner.QueryRowContext(ctx, "SELECT "+userColumns+" FROM `users` WHERE `id` = ? LIMIT 1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, notFound("用户不存在")
	}
	return user, err
}

func findUserByUsername(ctx context.Context, runner dbRunner, username string) (User, error) {
	return scanUser(runner.QueryRowContext(ctx, "SELECT "+userColumns+" FROM `users` WHERE `username` = ? LIMIT 1", username))
}

const shiftColumns = "`id`, `storeId`, `name`, `code`, `startTime`, `endTime`, `color`, `sortOrder`, `isOff`, `createdAt`, `updatedAt`"

func scanShift(row scanner) (ShiftTemplate, error) {
	var shift ShiftTemplate
	err := row.Scan(
		&shift.ID,
		&shift.StoreID,
		&shift.Name,
		&shift.Code,
		&shift.StartTime,
		&shift.EndTime,
		&shift.Color,
		&shift.SortOrder,
		&shift.IsOff,
		&shift.CreatedAt,
		&shift.UpdatedAt,
	)
	return shift, err
}

func findShift(ctx context.Context, runner dbRunner, storeID, id string) (ShiftTemplate, error) {
	shift, err := scanShift(runner.QueryRowContext(ctx, "SELECT "+shiftColumns+" FROM `shift_templates` WHERE `id` = ? AND `storeId` = ? LIMIT 1", id, storeID))
	if errors.Is(err, sql.ErrNoRows) {
		return ShiftTemplate{}, notFound("班次不存在")
	}
	return shift, err
}

const scheduleColumns = "`id`, `storeId`, `userId`, `workDate`, `shiftTemplateId`, `status`, `version`, `note`, `createdById`, `updatedById`, `createdAt`, `updatedAt`"

func scanSchedule(row scanner) (Schedule, error) {
	var item Schedule
	var workDate DBDate
	err := row.Scan(
		&item.ID,
		&item.StoreID,
		&item.UserID,
		&workDate,
		&item.ShiftTemplateID,
		&item.Status,
		&item.Version,
		&item.Note,
		&item.CreatedByID,
		&item.UpdatedByID,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	item.WorkDate = string(workDate)
	return item, err
}

func findSchedule(ctx context.Context, runner dbRunner, storeID, userID, workDate string) (Schedule, error) {
	return scanSchedule(runner.QueryRowContext(ctx, "SELECT "+scheduleColumns+" FROM `schedules` WHERE `storeId` = ? AND `userId` = ? AND `workDate` = ? LIMIT 1", storeID, userID, workDate))
}

const notificationColumns = "`id`, `storeId`, `userId`, `title`, `content`, `isRead`, `relatedType`, `relatedId`, `createdAt`"

func scanNotification(row scanner) (Notification, error) {
	var item Notification
	var relatedType, relatedID sql.NullString
	err := row.Scan(
		&item.ID,
		&item.StoreID,
		&item.UserID,
		&item.Title,
		&item.Content,
		&item.IsRead,
		&relatedType,
		&relatedID,
		&item.CreatedAt,
	)
	item.RelatedType = ptrFromNull(relatedType)
	item.RelatedID = ptrFromNull(relatedID)
	return item, err
}

const requestJoinQuery = `
SELECT
  r.` + "`id`" + `, r.` + "`storeId`" + `, r.` + "`type`" + `, r.` + "`status`" + `, r.` + "`requesterId`" + `,
  r.` + "`targetUserId`" + `, r.` + "`fromDate`" + `, r.` + "`toDate`" + `, r.` + "`fromShiftId`" + `, r.` + "`toShiftId`" + `,
  r.` + "`reason`" + `, r.` + "`reviewNote`" + `, r.` + "`approverId`" + `, r.` + "`reviewedAt`" + `, r.` + "`createdAt`" + `, r.` + "`updatedAt`" + `,
  u.` + "`id`" + `, u.` + "`username`" + `, u.` + "`displayName`" + `, u.` + "`role`" + `, u.` + "`storeId`" + `, u.` + "`isActive`" + `,
  t.` + "`id`" + `, t.` + "`username`" + `, t.` + "`displayName`" + `, t.` + "`role`" + `, t.` + "`storeId`" + `, t.` + "`isActive`" + `
FROM ` + "`schedule_requests`" + ` r
INNER JOIN ` + "`users`" + ` u ON u.` + "`id`" + ` = r.` + "`requesterId`" + ` AND u.` + "`storeId`" + ` = r.` + "`storeId`" + `
LEFT JOIN ` + "`users`" + ` t ON t.` + "`id`" + ` = r.` + "`targetUserId`" + ` AND t.` + "`storeId`" + ` = r.` + "`storeId`" + `
`

func scanRequest(row scanner) (ScheduleRequest, error) {
	var item ScheduleRequest
	var targetUserID, fromShiftID, toShiftID, reason, reviewNote, approverID, reviewedAt sql.NullString
	var toDate NullDBDate
	var fromDate DBDate
	var targetID, targetUsername, targetDisplayName, targetRole, targetStoreID sql.NullString
	var targetActive sql.NullBool
	err := row.Scan(
		&item.ID,
		&item.StoreID,
		&item.Type,
		&item.Status,
		&item.RequesterID,
		&targetUserID,
		&fromDate,
		&toDate,
		&fromShiftID,
		&toShiftID,
		&reason,
		&reviewNote,
		&approverID,
		&reviewedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Requester.ID,
		&item.Requester.Username,
		&item.Requester.DisplayName,
		&item.Requester.Role,
		&item.Requester.StoreID,
		&item.Requester.IsActive,
		&targetID,
		&targetUsername,
		&targetDisplayName,
		&targetRole,
		&targetStoreID,
		&targetActive,
	)
	if err != nil {
		return item, err
	}
	item.FromDate = string(fromDate)
	item.TargetUserID = ptrFromNull(targetUserID)
	if toDate.Valid {
		value := string(toDate.DBDate)
		item.ToDate = &value
	}
	item.FromShiftID = ptrFromNull(fromShiftID)
	item.ToShiftID = ptrFromNull(toShiftID)
	item.Reason = ptrFromNull(reason)
	item.ReviewNote = ptrFromNull(reviewNote)
	item.ApproverID = ptrFromNull(approverID)
	if reviewedAt.Valid {
		var parsed DBTime
		if err := parsed.Scan(reviewedAt.String); err != nil {
			return item, err
		}
		item.ReviewedAt = &parsed
	}
	if targetID.Valid {
		item.TargetUser = &User{
			ID:          targetID.String,
			Username:    targetUsername.String,
			DisplayName: targetDisplayName.String,
			Role:        targetRole.String,
			StoreID:     targetStoreID.String,
			IsActive:    targetActive.Bool,
		}
	}
	return item, nil
}

func findRequest(ctx context.Context, runner dbRunner, storeID, id string) (ScheduleRequest, error) {
	item, err := scanRequest(runner.QueryRowContext(ctx, requestJoinQuery+" WHERE r.`id` = ? AND r.`storeId` = ? LIMIT 1", id, storeID))
	if errors.Is(err, sql.ErrNoRows) {
		return ScheduleRequest{}, notFound("申请不存在")
	}
	return item, err
}

func isNotFoundError(err error) bool {
	var apiErr *apiError
	return errors.As(err, &apiErr) && apiErr.Status == 404
}

func parseDBTime(raw string) (DBTime, error) {
	var result DBTime
	if err := result.Scan(raw); err != nil {
		return DBTime{}, fmt.Errorf("parse database time: %w", err)
	}
	return result, nil
}

func monthNow() string {
	now := time.Now()
	return fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
}

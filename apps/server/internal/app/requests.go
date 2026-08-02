package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type createRequestInput struct {
	Type         string  `json:"type"`
	FromDate     string  `json:"fromDate"`
	ToDate       *string `json:"toDate"`
	TargetUserID *string `json:"targetUserId"`
	FromShiftID  *string `json:"fromShiftId"`
	ToShiftID    *string `json:"toShiftId"`
	Reason       *string `json:"reason"`
}

type reviewRequestInput struct {
	Action     *string `json:"action"`
	ReviewNote *string `json:"reviewNote"`
}

func validateCreateRequest(input createRequestInput) error {
	if input.Type != RequestLeave && input.Type != RequestSwap && input.Type != RequestChange {
		return badRequest("申请类型无效")
	}
	fromDate, err := parseISODate(input.FromDate)
	if err != nil {
		return err
	}
	if input.ToDate != nil {
		toDate, err := parseISODate(*input.ToDate)
		if err != nil {
			return err
		}
		if toDate.Before(fromDate) {
			return badRequest("结束日期不能早于开始日期")
		}
		if *input.ToDate != input.FromDate {
			return badRequest("暂不支持多日排班申请，请按天提交")
		}
	}
	if input.Type == RequestSwap && (input.TargetUserID == nil || strings.TrimSpace(*input.TargetUserID) == "") {
		return badRequest("调班申请必须指定目标员工")
	}
	if input.Type == RequestChange && (input.ToShiftID == nil || strings.TrimSpace(*input.ToShiftID) == "") {
		return badRequest("改班申请必须指定目标班次")
	}
	return nil
}

func requestTypeLabel(requestType string) string {
	switch requestType {
	case RequestLeave:
		return "调休"
	case RequestSwap:
		return "调班"
	default:
		return "改班"
	}
}

func (s *Server) createRequest(w http.ResponseWriter, r *http.Request) error {
	var input createRequestInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	if err := validateCreateRequest(input); err != nil {
		return err
	}
	claims := requestClaims(r)
	if claims.Role != RoleStaff && claims.Role != RoleManager {
		return forbidden("Forbidden")
	}
	if input.TargetUserID != nil {
		targetID := strings.TrimSpace(*input.TargetUserID)
		target, err := findUserInStore(r.Context(), s.db, claims.StoreID, targetID)
		if err != nil {
			if isNotFoundError(err) {
				return badRequest("目标员工不存在")
			}
			return err
		}
		if target.ID == claims.Subject {
			return badRequest("调班目标不能是自己")
		}
		if !target.IsActive {
			return badRequest("目标员工已停用")
		}
		input.TargetUserID = &targetID
	}
	for _, shiftID := range []*string{input.FromShiftID, input.ToShiftID} {
		if shiftID == nil || strings.TrimSpace(*shiftID) == "" {
			continue
		}
		if _, err := findShift(r.Context(), s.db, claims.StoreID, strings.TrimSpace(*shiftID)); err != nil {
			if isNotFoundError(err) {
				return badRequest("申请中的班次不存在")
			}
			return err
		}
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id := uuid.NewString()
	now := nowDB()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO `+"`schedule_requests`"+` (`+"`id`"+`, `+"`storeId`"+`, `+"`type`"+`, `+"`status`"+`, `+"`requesterId`"+`, `+"`targetUserId`"+`, `+"`fromDate`"+`, `+"`toDate`"+`, `+"`fromShiftId`"+`, `+"`toShiftId`"+`, `+"`reason`"+`, `+"`createdAt`"+`, `+"`updatedAt`"+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, claims.StoreID, input.Type, RequestPending, claims.Subject, nullableArg(input.TargetUserID), input.FromDate, nullableArg(input.ToDate), nullableArg(input.FromShiftID), nullableArg(input.ToShiftID), nullableArg(input.Reason), now, now)
	if err != nil {
		return err
	}
	item, err := findRequest(r.Context(), tx, claims.StoreID, id)
	if err != nil {
		return err
	}
	relatedType := "request"
	notifications, err := notifyManagers(r.Context(), tx, claims.StoreID, "新的排班申请", fmt.Sprintf("%s 提交了%s申请", item.Requester.DisplayName, requestTypeLabel(item.Type)), &relatedType, &item.ID)
	if err != nil {
		return err
	}
	settings, err := s.loadStorePushSettings(r.Context(), tx, claims.StoreID)
	if err != nil {
		return err
	}
	if settings.SwapRequestEnabled {
		content := fmt.Sprintf("%s 提交了%s申请\n日期：%s", item.Requester.DisplayName, requestTypeLabel(item.Type), item.FromDate)
		if item.TargetUser != nil {
			content += "\n调班对象：" + item.TargetUser.DisplayName
		}
		if item.ToShiftID != nil {
			targetShift, err := findShift(r.Context(), tx, claims.StoreID, strings.TrimSpace(*item.ToShiftID))
			if err != nil {
				return err
			}
			content += "\n目标班次：" + targetShift.Name
		}
		if item.Reason != nil && strings.TrimSpace(*item.Reason) != "" {
			content += "\n原因：" + strings.TrimSpace(*item.Reason)
		}
		if err := s.enqueuePushToActiveManagers(r.Context(), tx, claims.StoreID, pushEventScheduleRequest, item.ID, "新的排班申请", content); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, notification := range notifications {
		s.realtime.EmitToUser(notification.UserID, "notification.created", notificationPayload(notification))
	}
	s.realtime.EmitToStore(claims.StoreID, "request.created", map[string]any{"id": item.ID, "type": item.Type, "status": item.Status})
	writeJSON(w, http.StatusCreated, item)
	return nil
}

func (s *Server) listRequests(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	query := requestJoinQuery + " WHERE r.`storeId` = ?"
	args := []any{claims.StoreID}
	if status := r.URL.Query().Get("status"); status != "" {
		query += " AND r.`status` = ?"
		args = append(args, status)
	}
	if claims.Role == RoleStaff {
		query += " AND r.`requesterId` = ?"
		args = append(args, claims.Subject)
	}
	query += " ORDER BY r.`createdAt` DESC"
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]ScheduleRequest, 0)
	for rows.Next() {
		item, err := scanRequest(rows)
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

func (s *Server) clearResolvedRequests(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	statuses := []any{RequestApproved, RequestRejected, RequestCancelled}
	requestFilter := "`storeId` = ? AND `status` IN (?, ?, ?)"
	requestArgs := []any{claims.StoreID, statuses[0], statuses[1], statuses[2]}
	if claims.Role == RoleStaff {
		requestFilter += " AND `requesterId` = ?"
		requestArgs = append(requestArgs, claims.Subject)
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	notificationQuery := "DELETE FROM `notifications` WHERE `storeId` = ? AND `relatedType` = 'request' AND `relatedId` IN (SELECT `id` FROM `schedule_requests` WHERE " + requestFilter + ")"
	notificationArgs := append([]any{claims.StoreID}, requestArgs...)
	if _, err := tx.ExecContext(r.Context(), notificationQuery, notificationArgs...); err != nil {
		return err
	}
	deleted, err := execAffected(r.Context(), tx, "DELETE FROM `schedule_requests` WHERE "+requestFilter, requestArgs...)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if deleted > 0 {
		s.realtime.EmitToStore(claims.StoreID, "request.cleared", map[string]any{"storeId": claims.StoreID, "deleted": deleted, "by": claims.Subject})
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "deleted": deleted})
	return nil
}

func (s *Server) deleteRequest(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	item, err := findRequest(r.Context(), s.db, claims.StoreID, r.PathValue("id"))
	if err != nil {
		return err
	}
	if item.Status == RequestPending {
		return badRequest("待审批申请不能清理，请先处理")
	}
	if claims.Role == RoleStaff && item.RequesterID != claims.Subject {
		return forbidden("只能清理自己的申请记录")
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM `notifications` WHERE `storeId` = ? AND `relatedType` = 'request' AND `relatedId` = ?", claims.StoreID, item.ID); err != nil {
		return err
	}
	deleted, err := execAffected(r.Context(), tx, "DELETE FROM `schedule_requests` WHERE `id` = ? AND `storeId` = ?", item.ID, claims.StoreID)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.realtime.EmitToStore(claims.StoreID, "request.cleared", map[string]any{"storeId": claims.StoreID, "deleted": deleted, "id": item.ID, "by": claims.Subject})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": deleted})
	return nil
}

func (s *Server) approveRequest(w http.ResponseWriter, r *http.Request) error {
	return s.reviewRequest(w, r, true)
}

func (s *Server) rejectRequest(w http.ResponseWriter, r *http.Request) error {
	return s.reviewRequest(w, r, false)
}

func (s *Server) reviewRequest(w http.ResponseWriter, r *http.Request, approve bool) error {
	var input reviewRequestInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	if input.Action != nil {
		expectedAction := "reject"
		if approve {
			expectedAction = "approve"
		}
		if *input.Action != expectedAction {
			return badRequest("审批动作与请求路径不一致")
		}
	}
	claims := requestClaims(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	item, err := findRequest(r.Context(), tx, claims.StoreID, r.PathValue("id"))
	if err != nil {
		return err
	}
	if item.Status != RequestPending {
		return badRequest("该申请已处理")
	}
	if _, err := parseISODate(item.FromDate); err != nil {
		return badRequest("申请日期无效")
	}
	if _, err := findUserInStore(r.Context(), tx, claims.StoreID, item.RequesterID); err != nil {
		if isNotFoundError(err) {
			return badRequest("申请人不存在")
		}
		return err
	}

	status := RequestRejected
	if approve {
		requester, err := findUserInStore(r.Context(), tx, claims.StoreID, item.RequesterID)
		if err != nil {
			return err
		}
		if !requester.IsActive {
			return badRequest("申请人已停用")
		}
		status = RequestApproved
		switch item.Type {
		case RequestLeave:
			var offShiftID string
			err := tx.QueryRowContext(r.Context(), "SELECT `id` FROM `shift_templates` WHERE `storeId` = ? AND `isOff` = 1 LIMIT 1", claims.StoreID).Scan(&offShiftID)
			if err == sql.ErrNoRows {
				return badRequest("未配置休息班次")
			}
			if err != nil {
				return err
			}
			if err := applyLeave(r.Context(), tx, claims.StoreID, item.RequesterID, item.FromDate, offShiftID, claims.Subject); err != nil {
				return err
			}
		case RequestSwap:
			if item.TargetUserID == nil {
				return badRequest("调班申请缺少目标员工")
			}
			target, err := findUserInStore(r.Context(), tx, claims.StoreID, *item.TargetUserID)
			if err != nil {
				if isNotFoundError(err) {
					return badRequest("目标员工不存在")
				}
				return err
			}
			if !target.IsActive {
				return badRequest("目标员工已停用")
			}
			if err := applySwap(r.Context(), tx, claims.StoreID, item.RequesterID, *item.TargetUserID, item.FromDate, claims.Subject); err != nil {
				return err
			}
		case RequestChange:
			if item.ToShiftID == nil {
				return badRequest("改班申请缺少目标班次")
			}
			if err := applyChange(r.Context(), tx, claims.StoreID, item.RequesterID, item.FromDate, *item.ToShiftID, claims.Subject); err != nil {
				return err
			}
		}
	}

	reviewedAt := nowDB()
	result, err := tx.ExecContext(r.Context(), "UPDATE `schedule_requests` SET `status` = ?, `reviewNote` = ?, `approverId` = ?, `reviewedAt` = ?, `updatedAt` = ? WHERE `id` = ? AND `storeId` = ? AND `status` = ?", status, nullableArg(input.ReviewNote), claims.Subject, reviewedAt, reviewedAt, item.ID, claims.StoreID, RequestPending)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return badRequest("该申请已处理")
	}

	relatedType := "request"
	title := "申请已驳回"
	content := fmt.Sprintf("你的%s申请已被驳回", requestTypeLabel(item.Type))
	if approve {
		title = "申请已通过"
		content = fmt.Sprintf("你的%s申请已通过，排班已自动更新", requestTypeLabel(item.Type))
	}
	notifications := make([]Notification, 0, 2)
	notification, err := createNotification(r.Context(), tx, claims.StoreID, item.RequesterID, title, content, &relatedType, &item.ID)
	if err != nil {
		return err
	}
	notifications = append(notifications, notification)
	if approve && item.TargetUserID != nil {
		notification, err := createNotification(r.Context(), tx, claims.StoreID, *item.TargetUserID, "排班已调整", "与你相关的调班申请已通过，请查看最新排班", &relatedType, &item.ID)
		if err != nil {
			return err
		}
		notifications = append(notifications, notification)
	}
	updated, err := findRequest(r.Context(), tx, claims.StoreID, item.ID)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	if approve {
		s.realtime.EmitToStore(claims.StoreID, "schedule.updated", map[string]any{"month": item.FromDate[:7], "storeId": claims.StoreID, "reason": "request-approved", "requestId": item.ID})
	}
	s.realtime.EmitToStore(claims.StoreID, "request.resolved", map[string]any{"id": item.ID, "status": status})
	for _, notification := range notifications {
		s.realtime.EmitToUser(notification.UserID, "notification.created", notificationPayload(notification))
	}
	writeJSON(w, http.StatusCreated, updated)
	return nil
}

func notificationPayload(item Notification) map[string]any {
	return map[string]any{
		"id":          item.ID,
		"storeId":     item.StoreID,
		"userId":      item.UserID,
		"title":       item.Title,
		"content":     item.Content,
		"isRead":      item.IsRead,
		"relatedType": item.RelatedType,
		"relatedId":   item.RelatedID,
		"createdAt":   item.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

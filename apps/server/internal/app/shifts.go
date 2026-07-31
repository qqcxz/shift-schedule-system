package app

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

var timePattern = regexp.MustCompile(`^\d{2}:\d{2}$`)

type createShiftInput struct {
	Name      string  `json:"name"`
	Code      *string `json:"code"`
	StartTime string  `json:"startTime"`
	EndTime   string  `json:"endTime"`
	Color     *string `json:"color"`
	SortOrder *int    `json:"sortOrder"`
	IsOff     *bool   `json:"isOff"`
}

type updateShiftInput struct {
	Name      *string `json:"name"`
	Code      *string `json:"code"`
	StartTime *string `json:"startTime"`
	EndTime   *string `json:"endTime"`
	Color     *string `json:"color"`
	SortOrder *int    `json:"sortOrder"`
	IsOff     *bool   `json:"isOff"`
}

func normalizeShiftCode(name, code string) string {
	raw := code
	if strings.TrimSpace(raw) == "" {
		raw = name
	}
	raw = strings.ToUpper(strings.TrimSpace(raw))
	var result strings.Builder
	underscore := false
	for _, character := range raw {
		if unicode.IsSpace(character) {
			if !underscore && result.Len() > 0 {
				result.WriteRune('_')
				underscore = true
			}
			continue
		}
		allowed := character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character >= '\u4e00' && character <= '\u9fa5'
		if allowed {
			result.WriteRune(character)
			underscore = character == '_'
		}
	}
	runes := []rune(result.String())
	if len(runes) > 32 {
		runes = runes[:32]
	}
	if len(runes) == 0 {
		return "SHIFT_" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	}
	return string(runes)
}

func validateShiftCreate(input createShiftInput) error {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 32 {
		return badRequest("班次名称不能超过 32 个字符")
	}
	if input.Code != nil && len([]rune(*input.Code)) > 32 {
		return badRequest("班次编码不能超过 32 个字符")
	}
	isOff := input.IsOff != nil && *input.IsOff
	if err := validateShiftRange(input.StartTime, input.EndTime, isOff); err != nil {
		return err
	}
	if input.Color != nil && len([]rune(*input.Color)) > 20 {
		return badRequest("颜色值不能超过 20 个字符")
	}
	if input.SortOrder != nil && *input.SortOrder < 0 {
		return badRequest("排序值不能小于 0")
	}
	return nil
}

func validateShiftUpdate(input updateShiftInput) error {
	if input.Name != nil && (strings.TrimSpace(*input.Name) == "" || len([]rune(strings.TrimSpace(*input.Name))) > 32) {
		return badRequest("班次名称不能超过 32 个字符")
	}
	if input.Code != nil && len([]rune(*input.Code)) > 32 {
		return badRequest("班次编码不能超过 32 个字符")
	}
	if input.StartTime != nil && !validShiftTime(*input.StartTime) || input.EndTime != nil && !validShiftTime(*input.EndTime) {
		return badRequest("班次时间格式应为 HH:mm")
	}
	if input.Color != nil && len([]rune(*input.Color)) > 20 {
		return badRequest("颜色值不能超过 20 个字符")
	}
	if input.SortOrder != nil && *input.SortOrder < 0 {
		return badRequest("排序值不能小于 0")
	}
	return nil
}

func validShiftTime(value string) bool {
	if !timePattern.MatchString(value) {
		return false
	}
	hour, hourErr := strconv.Atoi(value[:2])
	minute, minuteErr := strconv.Atoi(value[3:])
	return hourErr == nil && minuteErr == nil && hour >= 0 && hour < 24 && minute >= 0 && minute < 60
}

func validateShiftRange(startTime, endTime string, isOff bool) error {
	if !validShiftTime(startTime) || !validShiftTime(endTime) {
		return badRequest("班次时间格式应为 HH:mm")
	}
	if isOff {
		return nil
	}
	startMinutes := 60*mustAtoi(startTime[:2]) + mustAtoi(startTime[3:])
	endMinutes := 60*mustAtoi(endTime[:2]) + mustAtoi(endTime[3:])
	if startMinutes == endMinutes {
		return badRequest("上班班次的开始和结束时间不能相同")
	}
	if endMinutes < startMinutes {
		return badRequest("暂不支持跨午夜班次，请将班次拆分到相邻日期")
	}
	return nil
}

func mustAtoi(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func ensureUniqueShiftCode(runner dbRunner, storeID, code, excludeID string, r *http.Request) error {
	query := "SELECT COUNT(*) FROM `shift_templates` WHERE `storeId` = ? AND `code` = ?"
	args := []any{storeID, code}
	if excludeID != "" {
		query += " AND `id` <> ?"
		args = append(args, excludeID)
	}
	var count int
	if err := runner.QueryRowContext(r.Context(), query, args...).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return badRequest("班次编码已存在")
	}
	return nil
}

func (s *Server) listShifts(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.QueryContext(r.Context(), "SELECT "+shiftColumns+" FROM `shift_templates` WHERE `storeId` = ? ORDER BY `sortOrder` ASC, `createdAt` ASC", requestClaims(r).StoreID)
	if err != nil {
		return err
	}
	defer rows.Close()
	shifts := make([]ShiftTemplate, 0)
	for rows.Next() {
		shift, err := scanShift(rows)
		if err != nil {
			return err
		}
		shifts = append(shifts, shift)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, shifts)
	return nil
}

func (s *Server) createShift(w http.ResponseWriter, r *http.Request) error {
	var input createShiftInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	if err := validateShiftCreate(input); err != nil {
		return err
	}
	claims := requestClaims(r)
	providedCode := ""
	if input.Code != nil {
		providedCode = *input.Code
	}
	code := normalizeShiftCode(input.Name, providedCode)
	if err := ensureUniqueShiftCode(s.db, claims.StoreID, code, "", r); err != nil {
		return err
	}
	sortOrder := 0
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	} else {
		if err := s.db.QueryRowContext(r.Context(), "SELECT COALESCE(MAX(`sortOrder`), 0) + 1 FROM `shift_templates` WHERE `storeId` = ?", claims.StoreID).Scan(&sortOrder); err != nil {
			return err
		}
	}
	isOff := input.IsOff != nil && *input.IsOff
	startTime, endTime := input.StartTime, input.EndTime
	if isOff {
		startTime, endTime = "00:00", "00:00"
	}
	color := "#409EFF"
	if isOff {
		color = "#909399"
	}
	if input.Color != nil && *input.Color != "" {
		color = *input.Color
	}
	id := uuid.NewString()
	now := nowDB()
	_, err := s.db.ExecContext(r.Context(), "INSERT INTO `shift_templates` (`id`, `storeId`, `name`, `code`, `startTime`, `endTime`, `color`, `sortOrder`, `isOff`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", id, claims.StoreID, strings.TrimSpace(input.Name), code, startTime, endTime, color, sortOrder, isOff, now, now)
	if err != nil {
		return err
	}
	shift, err := findShift(r.Context(), s.db, claims.StoreID, id)
	if err != nil {
		return err
	}
	s.realtime.EmitToStore(claims.StoreID, "shifts.updated", map[string]any{"action": "created", "shiftId": id})
	writeJSON(w, http.StatusCreated, shift)
	return nil
}

func (s *Server) updateShift(w http.ResponseWriter, r *http.Request) error {
	var input updateShiftInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	if err := validateShiftUpdate(input); err != nil {
		return err
	}
	claims := requestClaims(r)
	shift, err := findShift(r.Context(), s.db, claims.StoreID, r.PathValue("id"))
	if err != nil {
		return err
	}
	if input.Name != nil {
		shift.Name = strings.TrimSpace(*input.Name)
	}
	if input.Code != nil || input.Name != nil {
		code := shift.Code
		if input.Code != nil && *input.Code != "" {
			code = *input.Code
		}
		code = normalizeShiftCode(shift.Name, code)
		if err := ensureUniqueShiftCode(s.db, claims.StoreID, code, shift.ID, r); err != nil {
			return err
		}
		shift.Code = code
	}
	if input.StartTime != nil {
		shift.StartTime = *input.StartTime
	}
	if input.EndTime != nil {
		shift.EndTime = *input.EndTime
	}
	if input.Color != nil {
		shift.Color = *input.Color
	}
	if input.SortOrder != nil {
		shift.SortOrder = *input.SortOrder
	}
	if input.IsOff != nil {
		shift.IsOff = *input.IsOff
		if shift.IsOff {
			shift.StartTime, shift.EndTime = "00:00", "00:00"
		}
	}
	if err := validateShiftRange(shift.StartTime, shift.EndTime, shift.IsOff); err != nil {
		return err
	}
	_, err = s.db.ExecContext(r.Context(), "UPDATE `shift_templates` SET `name` = ?, `code` = ?, `startTime` = ?, `endTime` = ?, `color` = ?, `sortOrder` = ?, `isOff` = ?, `updatedAt` = ? WHERE `id` = ? AND `storeId` = ?", shift.Name, shift.Code, shift.StartTime, shift.EndTime, shift.Color, shift.SortOrder, shift.IsOff, nowDB(), shift.ID, claims.StoreID)
	if err != nil {
		return err
	}
	shift, err = findShift(r.Context(), s.db, claims.StoreID, shift.ID)
	if err != nil {
		return err
	}
	s.realtime.EmitToStore(claims.StoreID, "shifts.updated", map[string]any{"action": "updated", "shiftId": shift.ID})
	writeJSON(w, http.StatusOK, shift)
	return nil
}

func (s *Server) deleteShift(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	shift, err := findShift(r.Context(), s.db, claims.StoreID, r.PathValue("id"))
	if err != nil {
		return err
	}
	var usedCount int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM `schedules` WHERE `storeId` = ? AND `shiftTemplateId` = ?", claims.StoreID, shift.ID).Scan(&usedCount); err != nil {
		return err
	}
	if usedCount > 0 {
		return badRequest("该班次已用于排班，无法删除。可改为修改时间，或先调整相关排班")
	}
	var remaining int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM `shift_templates` WHERE `storeId` = ?", claims.StoreID).Scan(&remaining); err != nil {
		return err
	}
	if remaining <= 1 {
		return badRequest("至少保留一个班次")
	}
	result, err := s.db.ExecContext(r.Context(), "DELETE FROM `shift_templates` WHERE `id` = ? AND `storeId` = ?", shift.ID, claims.StoreID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return notFound("班次不存在")
	}
	s.realtime.EmitToStore(claims.StoreID, "shifts.updated", map[string]any{"action": "deleted", "shiftId": shift.ID})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	return nil
}

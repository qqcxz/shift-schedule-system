package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

const (
	exportMatrixSheet  = "月度排班表"
	exportLegendSheet  = "班次说明"
	excelFallbackColor = "909399"
)

var (
	excelHexColorPattern = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
	excelFileNamePattern = regexp.MustCompile(`^\d{4}-\d{2}$`)
)

type scheduleExportData struct {
	Store  Store
	Month  string
	Users  []User
	Shifts []ShiftTemplate
	Items  []ScheduleItem
}

func (s *Server) exportMonthSchedules(w http.ResponseWriter, r *http.Request) error {
	data, err := s.monthScheduleExportData(r.Context(), requestClaims(r).StoreID, r.URL.Query().Get("month"))
	if err != nil {
		return err
	}
	workbook, err := buildScheduleWorkbook(data)
	if err != nil {
		return err
	}
	filename, fallback := exportFileName(data.Store.Name, data.Month)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback, url.PathEscape(filename)))
	w.Header().Set("Content-Length", strconv.Itoa(len(workbook)))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	_, _ = w.Write(workbook)
	return nil
}

func (s *Server) monthScheduleExportData(ctx context.Context, storeID, month string) (scheduleExportData, error) {
	start, end, err := monthRange(month)
	if err != nil {
		return scheduleExportData{}, err
	}

	data := scheduleExportData{
		Month:  month,
		Users:  make([]User, 0),
		Shifts: make([]ShiftTemplate, 0),
		Items:  make([]ScheduleItem, 0),
	}

	var address sql.NullString
	err = s.db.QueryRowContext(ctx, "SELECT `id`, `name`, `address`, `createdAt`, `updatedAt` FROM `stores` WHERE `id` = ? LIMIT 1", storeID).Scan(
		&data.Store.ID,
		&data.Store.Name,
		&address,
		&data.Store.CreatedAt,
		&data.Store.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return scheduleExportData{}, notFound("门店不存在")
	}
	if err != nil {
		return scheduleExportData{}, err
	}
	data.Store.Address = ptrFromNull(address)

	userRows, err := s.db.QueryContext(ctx, "SELECT `id`, `displayName`, `role`, `storeId`, `isActive` FROM `users` WHERE `storeId` = ? AND `isActive` = 1 ORDER BY `role` ASC, `displayName` ASC", storeID)
	if err != nil {
		return scheduleExportData{}, err
	}
	for userRows.Next() {
		var user User
		if err := userRows.Scan(&user.ID, &user.DisplayName, &user.Role, &user.StoreID, &user.IsActive); err != nil {
			userRows.Close()
			return scheduleExportData{}, err
		}
		data.Users = append(data.Users, user)
	}
	if err := userRows.Close(); err != nil {
		return scheduleExportData{}, err
	}
	if err := userRows.Err(); err != nil {
		return scheduleExportData{}, err
	}

	shiftRows, err := s.db.QueryContext(ctx, "SELECT "+shiftColumns+" FROM `shift_templates` WHERE `storeId` = ? ORDER BY `sortOrder` ASC, `createdAt` ASC", storeID)
	if err != nil {
		return scheduleExportData{}, err
	}
	for shiftRows.Next() {
		shift, err := scanShift(shiftRows)
		if err != nil {
			shiftRows.Close()
			return scheduleExportData{}, err
		}
		data.Shifts = append(data.Shifts, shift)
	}
	if err := shiftRows.Close(); err != nil {
		return scheduleExportData{}, err
	}
	if err := shiftRows.Err(); err != nil {
		return scheduleExportData{}, err
	}

	scheduleRows, err := s.db.QueryContext(ctx, `
SELECT s.`+"`id`"+`, s.`+"`userId`"+`, u.`+"`displayName`"+`, s.`+"`workDate`"+`, s.`+"`shiftTemplateId`"+`,
       sh.`+"`name`"+`, sh.`+"`color`"+`, s.`+"`status`"+`, s.`+"`version`"+`, s.`+"`note`"+`
FROM `+"`schedules`"+` s
INNER JOIN `+"`users`"+` u ON u.`+"`id`"+` = s.`+"`userId`"+` AND u.`+"`storeId`"+` = s.`+"`storeId`"+` AND u.`+"`isActive`"+` = 1
INNER JOIN `+"`shift_templates`"+` sh ON sh.`+"`id`"+` = s.`+"`shiftTemplateId`"+` AND sh.`+"`storeId`"+` = s.`+"`storeId`"+`
WHERE s.`+"`storeId`"+` = ? AND s.`+"`workDate`"+` BETWEEN ? AND ?
ORDER BY s.`+"`userId`"+` ASC, s.`+"`workDate`"+` ASC`, storeID, start, end)
	if err != nil {
		return scheduleExportData{}, err
	}
	for scheduleRows.Next() {
		var item ScheduleItem
		var workDate DBDate
		var note sql.NullString
		if err := scheduleRows.Scan(&item.ID, &item.UserID, &item.UserName, &workDate, &item.ShiftTemplateID, &item.ShiftName, &item.ShiftColor, &item.Status, &item.Version, &note); err != nil {
			scheduleRows.Close()
			return scheduleExportData{}, err
		}
		item.WorkDate = string(workDate)
		item.Note = ptrFromNull(note)
		data.Items = append(data.Items, item)
	}
	if err := scheduleRows.Close(); err != nil {
		return scheduleExportData{}, err
	}
	if err := scheduleRows.Err(); err != nil {
		return scheduleExportData{}, err
	}

	return data, nil
}

func buildScheduleWorkbook(data scheduleExportData) ([]byte, error) {
	startText, endText, err := monthRange(data.Month)
	if err != nil {
		return nil, err
	}
	firstDate, err := time.ParseInLocation("2006-01-02", startText, time.UTC)
	if err != nil {
		return nil, err
	}
	lastDate, err := time.ParseInLocation("2006-01-02", endText, time.UTC)
	if err != nil {
		return nil, err
	}
	dates := make([]time.Time, 0, lastDate.Day())
	for date := firstDate; !date.After(lastDate); date = date.AddDate(0, 0, 1) {
		dates = append(dates, date)
	}

	book := excelize.NewFile()
	if _, err := book.NewSheet(exportMatrixSheet); err != nil {
		return nil, err
	}
	if err := book.DeleteSheet("Sheet1"); err != nil {
		return nil, err
	}
	if _, err := book.NewSheet(exportLegendSheet); err != nil {
		return nil, err
	}
	book.SetActiveSheet(0)

	if err := buildScheduleMatrix(book, data, dates); err != nil {
		return nil, err
	}
	if err := buildScheduleLegend(book, data.Shifts); err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	if err := book.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildScheduleMatrix(book *excelize.File, data scheduleExportData, dates []time.Time) error {
	lastDateColumn := excelColumnName(2 + len(dates))
	if err := book.MergeCell(exportMatrixSheet, "A1", lastDateColumn+"1"); err != nil {
		return err
	}
	monthDate, err := time.ParseInLocation("2006-01", data.Month, time.UTC)
	if err != nil {
		return err
	}
	title := fmt.Sprintf("%s %04d年%02d月 排班表", data.Store.Name, monthDate.Year(), monthDate.Month())
	if err := setExportText(book, exportMatrixSheet, "A1", title); err != nil {
		return err
	}

	titleStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"5B9BD5"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 16},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return err
	}
	headerStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2F75B5"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	weekendHeaderStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"DDEBF7"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Color: "1F4E78"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	nameStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return err
	}
	roleStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return err
	}
	dataStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	weekendDataStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F4F9FD"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	if err := book.SetCellStyle(exportMatrixSheet, "A1", lastDateColumn+"1", titleStyle); err != nil {
		return err
	}
	for column, value := range []string{"员工", "角色"} {
		cell := fmt.Sprintf("%s2", excelColumnName(column+1))
		if err := setExportText(book, exportMatrixSheet, cell, value); err != nil {
			return err
		}
		if err := book.SetCellStyle(exportMatrixSheet, cell, cell, headerStyle); err != nil {
			return err
		}
	}
	for index, date := range dates {
		column := excelColumnName(index + 3)
		cell := column + "2"
		value := fmt.Sprintf("%02d\n%s", date.Day(), exportWeekdayLabel(date.Weekday()))
		if err := setExportText(book, exportMatrixSheet, cell, value); err != nil {
			return err
		}
		style := headerStyle
		if isWeekend(date) {
			style = weekendHeaderStyle
		}
		if err := book.SetCellStyle(exportMatrixSheet, cell, cell, style); err != nil {
			return err
		}
	}

	items := make(map[string]ScheduleItem, len(data.Items))
	for _, item := range data.Items {
		items[item.UserID+"\x00"+item.WorkDate] = item
	}
	shiftNames := make(map[string]string, len(data.Shifts))
	shiftColors := make(map[string]string, len(data.Shifts))
	for _, shift := range data.Shifts {
		shiftNames[shift.ID] = shift.Name
		shiftColors[shift.ID] = shift.Color
	}
	shiftStyles := make(map[string]int)
	row := 3
	for _, user := range data.Users {
		if !user.IsActive {
			continue
		}
		if err := setExportText(book, exportMatrixSheet, fmt.Sprintf("A%d", row), user.DisplayName); err != nil {
			return err
		}
		if err := book.SetCellStyle(exportMatrixSheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), nameStyle); err != nil {
			return err
		}
		if err := setExportText(book, exportMatrixSheet, fmt.Sprintf("B%d", row), excelRoleLabel(user.Role)); err != nil {
			return err
		}
		if err := book.SetCellStyle(exportMatrixSheet, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), roleStyle); err != nil {
			return err
		}
		for dateIndex, date := range dates {
			column := excelColumnName(dateIndex + 3)
			cell := fmt.Sprintf("%s%d", column, row)
			item, found := items[user.ID+"\x00"+date.Format("2006-01-02")]
			if !found {
				if err := book.SetCellStr(exportMatrixSheet, cell, "-"); err != nil {
					return err
				}
				style := dataStyle
				if isWeekend(date) {
					style = weekendDataStyle
				}
				if err := book.SetCellStyle(exportMatrixSheet, cell, cell, style); err != nil {
					return err
				}
				continue
			}
			shiftName := item.ShiftName
			if shiftName == "" {
				shiftName = shiftNames[item.ShiftTemplateID]
			}
			if err := setExportText(book, exportMatrixSheet, cell, shiftName); err != nil {
				return err
			}
			color := item.ShiftColor
			if color == "" {
				color = shiftColors[item.ShiftTemplateID]
			}
			normalized := normalizedExcelColor(color)
			style, exists := shiftStyles[normalized]
			if !exists {
				style, err = book.NewStyle(&excelize.Style{
					Border:    exportCellBorders(),
					Fill:      excelize.Fill{Type: "pattern", Color: []string{normalized}, Pattern: 1},
					Font:      &excelize.Font{Color: "FFFFFF"},
					Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
				})
				if err != nil {
					return err
				}
				shiftStyles[normalized] = style
			}
			if err := book.SetCellStyle(exportMatrixSheet, cell, cell, style); err != nil {
				return err
			}
		}
		row++
	}

	if err := book.SetColWidth(exportMatrixSheet, "A", "A", 18); err != nil {
		return err
	}
	if err := book.SetColWidth(exportMatrixSheet, "B", "B", 10); err != nil {
		return err
	}
	if len(dates) > 0 {
		if err := book.SetColWidth(exportMatrixSheet, "C", lastDateColumn, 11); err != nil {
			return err
		}
	}
	if err := book.SetRowHeight(exportMatrixSheet, 1, 30); err != nil {
		return err
	}
	if err := book.SetRowHeight(exportMatrixSheet, 2, 34); err != nil {
		return err
	}
	if err := book.SetPanes(exportMatrixSheet, &excelize.Panes{
		Freeze:      true,
		XSplit:      2,
		YSplit:      2,
		TopLeftCell: "C3",
		ActivePane:  "bottomRight",
		Selection:   []excelize.Selection{{SQRef: "C3", ActiveCell: "C3", Pane: "bottomRight"}},
	}); err != nil {
		return err
	}
	setOptionalPageLayout(book, exportMatrixSheet, "$A$1:$"+lastDateColumn+fmt.Sprintf("$%d", row-1))
	return nil
}

func buildScheduleLegend(book *excelize.File, shifts []ShiftTemplate) error {
	headerStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2F75B5"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	dataStyle, err := book.NewStyle(&excelize.Style{
		Border:    exportCellBorders(),
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return err
	}
	colorStyles := make(map[string]int)
	for column, value := range []string{"班次名称", "班次编码", "开始时间", "结束时间", "类型", "颜色"} {
		cell := fmt.Sprintf("%s1", excelColumnName(column+1))
		if err := setExportText(book, exportLegendSheet, cell, value); err != nil {
			return err
		}
		if err := book.SetCellStyle(exportLegendSheet, cell, cell, headerStyle); err != nil {
			return err
		}
	}
	for index, shift := range shifts {
		row := index + 2
		startTime, endTime := shift.StartTime, shift.EndTime
		shiftType := "上班"
		if shift.IsOff {
			startTime, endTime, shiftType = "休息", "休息", "休息"
		}
		values := []string{shift.Name, shift.Code, startTime, endTime, shiftType, shift.Color}
		for column, value := range values {
			cell := fmt.Sprintf("%s%d", excelColumnName(column+1), row)
			if err := setExportText(book, exportLegendSheet, cell, value); err != nil {
				return err
			}
			style := dataStyle
			if column == 5 {
				normalized := normalizedExcelColor(shift.Color)
				var exists bool
				style, exists = colorStyles[normalized]
				if !exists {
					style, err = book.NewStyle(&excelize.Style{
						Border:    exportCellBorders(),
						Fill:      excelize.Fill{Type: "pattern", Color: []string{normalized}, Pattern: 1},
						Font:      &excelize.Font{Color: "FFFFFF"},
						Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
					})
					if err != nil {
						return err
					}
					colorStyles[normalized] = style
				}
			}
			if err := book.SetCellStyle(exportLegendSheet, cell, cell, style); err != nil {
				return err
			}
		}
	}

	for column, width := range []float64{18, 16, 12, 12, 10, 16} {
		if err := book.SetColWidth(exportLegendSheet, excelColumnName(column+1), excelColumnName(column+1), width); err != nil {
			return err
		}
	}
	if err := book.SetRowHeight(exportLegendSheet, 1, 28); err != nil {
		return err
	}
	if err := book.SetPanes(exportLegendSheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
		Selection:   []excelize.Selection{{SQRef: "A2", ActiveCell: "A2", Pane: "bottomLeft"}},
	}); err != nil {
		return err
	}
	setOptionalPageLayout(book, exportLegendSheet, "$A$1:$F$"+fmt.Sprintf("%d", len(shifts)+1))
	return nil
}

func setOptionalPageLayout(book *excelize.File, sheet, printArea string) {
	landscape := "landscape"
	fitWidth := 1
	fitHeight := 0
	_ = book.SetPageLayout(sheet, &excelize.PageLayoutOptions{Orientation: &landscape, FitToWidth: &fitWidth, FitToHeight: &fitHeight})
	fitToPage := true
	_ = book.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: &fitToPage})
	_ = book.SetDefinedName(&excelize.DefinedName{
		Name:     "_xlnm.Print_Area",
		RefersTo: fmt.Sprintf("'%s'!%s", sheet, printArea),
		Scope:    sheet,
	})
	if sheet == exportMatrixSheet {
		_ = book.SetDefinedName(&excelize.DefinedName{
			Name:     "_xlnm.Print_Titles",
			RefersTo: fmt.Sprintf("'%s'!$1:$2", sheet),
			Scope:    sheet,
		})
	}
}

func exportCellBorders() []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: "D9E2F3", Style: 1},
		{Type: "top", Color: "D9E2F3", Style: 1},
		{Type: "bottom", Color: "D9E2F3", Style: 1},
		{Type: "right", Color: "D9E2F3", Style: 1},
	}
}

func setExportText(book *excelize.File, sheet, cell, value string) error {
	return book.SetCellStr(sheet, cell, escapedExportText(value))
}

func escapedExportText(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func excelColumnName(number int) string {
	if number <= 0 {
		return ""
	}
	var result [8]byte
	index := len(result)
	for number > 0 {
		number--
		index--
		result[index] = byte('A' + number%26)
		number /= 26
	}
	return string(result[index:])
}

func normalizedExcelColor(raw string) string {
	color := strings.TrimSpace(raw)
	color = strings.TrimPrefix(color, "#")
	if len(color) == 3 && isHexColor(color) {
		color = string([]byte{color[0], color[0], color[1], color[1], color[2], color[2]})
	}
	if !excelHexColorPattern.MatchString(color) {
		return excelFallbackColor
	}
	return strings.ToUpper(color)
}

func isHexColor(value string) bool {
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func exportFileName(storeName, month string) (safeUTF8Name, asciiFallback string) {
	cleanStoreName := sanitizeExportFileName(storeName)
	if cleanStoreName == "" {
		cleanStoreName = "门店"
	}
	if !excelFileNamePattern.MatchString(month) {
		month = "schedule"
	}
	if month == "schedule" {
		return cleanStoreName + "-排班表.xlsx", "schedule.xlsx"
	}
	return fmt.Sprintf("%s-%s年%s月排班表.xlsx", cleanStoreName, month[:4], month[5:]), month + "-schedule.xlsx"
}

func sanitizeExportFileName(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if unicode.IsControl(character) || strings.ContainsRune(`<>:"/\\|?*`, character) {
			builder.WriteRune('_')
			continue
		}
		builder.WriteRune(character)
	}
	return strings.TrimRight(builder.String(), " .")
}

func excelRoleLabel(role string) string {
	switch role {
	case RoleManager:
		return "店长"
	case RoleStaff:
		return "店员"
	default:
		return role
	}
}

func exportWeekdayLabel(day time.Weekday) string {
	return [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[day]
}

func isWeekend(date time.Time) bool {
	return date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
}

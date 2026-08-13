package app

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestBuildScheduleWorkbookContainsMonthlyMatrixAndLegend(t *testing.T) {
	data := scheduleExportData{
		Store: Store{ID: "store-1", Name: "示例门店"},
		Month: "2026-08",
		Users: []User{
			{ID: "manager", DisplayName: "店长", Role: RoleManager, IsActive: true},
			{ID: "staff", DisplayName: "员工一", Role: RoleStaff, IsActive: true},
		},
		Shifts: []ShiftTemplate{
			{ID: "morning", Name: "早班", Code: "MORNING", StartTime: "09:00", EndTime: "17:00", Color: "#409EFF"},
			{ID: "off", Name: "休息", Code: "OFF", IsOff: true, Color: "#909399"},
		},
		Items: []ScheduleItem{
			{UserID: "staff", UserName: "员工一", WorkDate: "2026-08-01", ShiftTemplateID: "morning", ShiftName: "早班", ShiftColor: "#409EFF"},
		},
	}

	raw, err := buildScheduleWorkbook(data)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()

	if got := book.GetSheetList(); !reflect.DeepEqual(got, []string{"月度排班表", "班次说明"}) {
		t.Fatalf("sheets = %#v", got)
	}
	if got, err := book.GetCellValue("月度排班表", "A1"); err != nil || !strings.Contains(got, "示例门店") {
		t.Fatalf("title = %q, err = %v", got, err)
	}
	if got, err := book.GetCellValue("月度排班表", "C4"); err != nil || got != "早班" {
		t.Fatalf("schedule cell = %q, err = %v", got, err)
	}
	if got, err := book.GetCellValue("月度排班表", "C3"); err != nil || got != "-" {
		t.Fatalf("manager placeholder = %q, err = %v", got, err)
	}
	if got, err := book.GetCellValue("班次说明", "A2"); err != nil || got != "早班" {
		t.Fatalf("legend = %q, err = %v", got, err)
	}
	if got, err := book.GetCellValue("班次说明", "C3"); err != nil || got != "休息" {
		t.Fatalf("off shift start time = %q, err = %v", got, err)
	}
}

func TestBuildScheduleWorkbookEscapesFormulaTextAndUsesFallbackColor(t *testing.T) {
	raw, err := buildScheduleWorkbook(scheduleExportData{
		Store: Store{ID: "store-1", Name: "=危险门店"},
		Month: "2026-08",
		Users: []User{
			{ID: "staff", DisplayName: "+员工", Role: RoleStaff, IsActive: true},
		},
		Shifts: []ShiftTemplate{
			{ID: "shift", Name: "@班次", Code: "-CODE", StartTime: "09:00", EndTime: "17:00", Color: "invalid"},
		},
		Items: []ScheduleItem{
			{UserID: "staff", WorkDate: "2026-08-01", ShiftName: "@班次", ShiftColor: "invalid"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()

	for _, cell := range []struct {
		sheet string
		cell  string
		want  string
	}{
		{sheet: "月度排班表", cell: "A3", want: "'+员工"},
		{sheet: "月度排班表", cell: "C3", want: "'@班次"},
		{sheet: "班次说明", cell: "A2", want: "'@班次"},
		{sheet: "班次说明", cell: "B2", want: "'-CODE"},
	} {
		got, err := book.GetCellValue(cell.sheet, cell.cell)
		if err != nil || got != cell.want {
			t.Fatalf("%s!%s = %q, err = %v, want %q", cell.sheet, cell.cell, got, err, cell.want)
		}
	}
	title, err := book.GetCellValue("月度排班表", "A1")
	if err != nil || !strings.HasPrefix(title, "'=危险门店") || !strings.Contains(title, "2026年08月") {
		t.Fatalf("escaped title = %q, err = %v", title, err)
	}

	styleID, err := book.GetCellStyle("月度排班表", "C3")
	if err != nil {
		t.Fatal(err)
	}
	style, err := book.GetStyle(styleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(style.Fill.Color) == 0 || style.Fill.Color[0] != "909399" {
		t.Fatalf("fallback fill = %#v", style.Fill.Color)
	}
}

func TestMonthScheduleExportDataScopesRowsOrdersActiveUsersAndSupportsEmptyMonths(t *testing.T) {
	service := newScheduleExportTestServer(t)
	defer service.Close()
	ctx := context.Background()
	now := nowDB()

	const (
		storeID      = "store-1"
		otherStoreID = "store-2"
	)
	insertScheduleExportStore(t, ctx, service, storeID, "示例门店", now)
	insertScheduleExportStore(t, ctx, service, otherStoreID, "其他门店", now)
	insertScheduleExportUser(t, ctx, service, "staff-z", "staff-z", "员工乙", RoleStaff, storeID, true, now)
	insertScheduleExportUser(t, ctx, service, "manager-a", "manager-a", "店长甲", RoleManager, storeID, true, now)
	insertScheduleExportUser(t, ctx, service, "staff-a", "staff-a", "员工甲", RoleStaff, storeID, true, now)
	insertScheduleExportUser(t, ctx, service, "inactive", "inactive", "已停用", RoleStaff, storeID, false, now)
	insertScheduleExportUser(t, ctx, service, "other-user", "other-user", "其他员工", RoleStaff, otherStoreID, true, now)
	insertScheduleExportShift(t, ctx, service, "shift-1", storeID, "早班", "MORNING", 1, now)
	insertScheduleExportShift(t, ctx, service, "other-shift", otherStoreID, "其他班", "OTHER", 1, now)
	insertScheduleExportItem(t, ctx, service, "item-1", storeID, "staff-a", "2026-08-02", "shift-1", now)
	insertScheduleExportItem(t, ctx, service, "other-item", otherStoreID, "other-user", "2026-08-02", "other-shift", now)

	data, err := service.monthScheduleExportData(ctx, storeID, "2026-08")
	if err != nil {
		t.Fatal(err)
	}
	if data.Store.ID != storeID || data.Store.Name != "示例门店" || data.Month != "2026-08" {
		t.Fatalf("unexpected export identity: %+v", data)
	}
	if got := userIDs(data.Users); !reflect.DeepEqual(got, []string{"manager-a", "staff-z", "staff-a"}) {
		t.Fatalf("active users = %#v", got)
	}
	if len(data.Shifts) != 1 || data.Shifts[0].ID != "shift-1" {
		t.Fatalf("shifts = %#v", data.Shifts)
	}
	if len(data.Items) != 1 || data.Items[0].UserID != "staff-a" || data.Items[0].WorkDate != "2026-08-02" || data.Items[0].ShiftName != "早班" {
		t.Fatalf("items = %#v", data.Items)
	}

	emptyMonth, err := service.monthScheduleExportData(ctx, storeID, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if emptyMonth.Month != "2026-09" || !reflect.DeepEqual(userIDs(emptyMonth.Users), []string{"manager-a", "staff-z", "staff-a"}) {
		t.Fatalf("empty-month users = %#v, month = %q", userIDs(emptyMonth.Users), emptyMonth.Month)
	}
	if emptyMonth.Items == nil || len(emptyMonth.Items) != 0 {
		t.Fatalf("empty-month items = %#v", emptyMonth.Items)
	}
}

func newScheduleExportTestServer(t *testing.T) *Server {
	t.Helper()
	service, err := New(context.Background(), Config{
		DBType:             "sqlite",
		DBDatabase:         fmt.Sprintf("%s/schedule-export.sqlite", t.TempDir()),
		JWTSecret:          "test-secret",
		ScheduleKeepMonths: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func insertScheduleExportStore(t *testing.T, ctx context.Context, service *Server, id, name, now string) {
	t.Helper()
	if _, err := service.db.ExecContext(ctx, "INSERT INTO `stores` (`id`, `name`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?)", id, name, now, now); err != nil {
		t.Fatal(err)
	}
}

func insertScheduleExportUser(t *testing.T, ctx context.Context, service *Server, id, username, displayName, role, storeID string, active bool, now string) {
	t.Helper()
	if _, err := service.db.ExecContext(ctx, "INSERT INTO `users` (`id`, `username`, `displayName`, `passwordHash`, `role`, `storeId`, `isActive`, `authVersion`, `mustChangePassword`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, 1, 0, ?, ?)", id, username, displayName, "unused", role, storeID, active, now, now); err != nil {
		t.Fatal(err)
	}
}

func insertScheduleExportShift(t *testing.T, ctx context.Context, service *Server, id, storeID, name, code string, sortOrder int, now string) {
	t.Helper()
	if _, err := service.db.ExecContext(ctx, "INSERT INTO `shift_templates` (`id`, `storeId`, `name`, `code`, `startTime`, `endTime`, `color`, `sortOrder`, `isOff`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, '09:00', '17:00', '#409EFF', ?, 0, ?, ?)", id, storeID, name, code, sortOrder, now, now); err != nil {
		t.Fatal(err)
	}
}

func insertScheduleExportItem(t *testing.T, ctx context.Context, service *Server, id, storeID, userID, workDate, shiftID, now string) {
	t.Helper()
	if _, err := service.db.ExecContext(ctx, "INSERT INTO `schedules` (`id`, `storeId`, `userId`, `workDate`, `shiftTemplateId`, `status`, `version`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, 'normal', 1, ?, ?)", id, storeID, userID, workDate, shiftID, now, now); err != nil {
		t.Fatal(err)
	}
}

func userIDs(users []User) []string {
	ids := make([]string, len(users))
	for index, user := range users {
		ids[index] = user.ID
	}
	return ids
}

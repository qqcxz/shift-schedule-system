# Monthly Schedule Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an authenticated Web download that generates a styled `.xlsx` workbook for the selected store month, with a monthly schedule matrix and shift legend.

**Architecture:** The Go API owns the export data query, store isolation, filename headers, and in-memory Excel generation. The Vue schedule page calls a dedicated Blob API only after blocking unsaved manager drafts, then creates and releases a browser object URL for the download. Existing JSON endpoints, schedule persistence, publishing, and mini-program behavior remain unchanged.

**Tech Stack:** Go 1.24.1, `github.com/xuri/excelize/v2 v2.8.1`, `net/http`, SQLite/MySQL through `database/sql`, Vue 3 + TypeScript, Axios, Element Plus.

## Global Constraints

- Export format is `.xlsx` only; do not add PDF or CSV.
- Export is Web-only; do not add a mini-program UI or mini-program API wrapper.
- Both `manager` and `staff` roles may export the current store's full active-employee month view.
- Export reads only persisted database rows; unsaved manager drafts must be blocked in the Web UI.
- Every backend query must be constrained by the authenticated `storeId`; no client-supplied store or user scope is accepted.
- Generate the workbook in memory and do not create server-side temporary files or export-history records.
- Use the existing `monthRange`, `authenticate`, `writeError`, and Axios authentication patterns.
- Keep all edited files ASCII unless an existing file already contains the required Chinese UI/content strings.

---

### Task 1: Add the Excel dependency and backend export model/query

**Files:**
- Modify: `apps/server/go.mod`
- Modify: `apps/server/go.sum`
- Create: `apps/server/internal/app/schedule_export.go`
- Test: `apps/server/internal/app/schedule_export_test.go`

**Interfaces:**
- Consumes: `monthRange`, `requestClaims`, `Server.db`, `Store`, `User`, `ShiftTemplate`, `ScheduleItem`, and the existing `dbRunner`/`DBDate` conventions.
- Produces: `func (s *Server) exportMonthSchedules(w http.ResponseWriter, r *http.Request) error`, `func (s *Server) monthScheduleExportData(ctx context.Context, storeID, month string) (scheduleExportData, error)`, and `func buildScheduleWorkbook(data scheduleExportData) ([]byte, error)`.

- [ ] **Step 1: Add the dependency without changing application code**

Run:

```powershell
go get github.com/xuri/excelize/v2@v2.8.1
go mod tidy
```

Expected: `apps/server/go.mod` directly requires `github.com/xuri/excelize/v2 v2.8.1`, `go.sum` gains its checksums, and no unrelated dependency is upgraded beyond what `go mod tidy` requires.

- [ ] **Step 2: Write failing unit tests for export data ordering and workbook structure**

Create test fixtures with `New` and a temporary SQLite database, then add tests equivalent to:

```go
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
		Items: []ScheduleItem{{UserID: "staff", UserName: "员工一", WorkDate: "2026-08-01", ShiftTemplateID: "morning", ShiftName: "早班", ShiftColor: "#409EFF"}},
	}
	raw, err := buildScheduleWorkbook(data)
	if err != nil { t.Fatal(err) }
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil { t.Fatal(err) }
	if got := book.GetSheetList(); !reflect.DeepEqual(got, []string{"月度排班表", "班次说明"}) { t.Fatalf("sheets = %#v", got) }
	if got, _ := book.GetCellValue("月度排班表", "A1"); !strings.Contains(got, "示例门店") { t.Fatalf("title = %q", got) }
	if got, _ := book.GetCellValue("月度排班表", "C3"); got != "早班" { t.Fatalf("schedule cell = %q", got) }
	if got, _ := book.GetCellValue("班次说明", "A2"); got != "早班" { t.Fatalf("legend = %q", got) }
}
```

Also add a data-query test that inserts a second store and inactive user, calls `monthScheduleExportData`, and asserts that only active users and rows from the requested store are returned. Use a month with no schedule rows to assert the function still returns active users and a valid month.

- [ ] **Step 3: Run the focused tests and verify they fail for the missing implementation**

Run:

```powershell
go test ./internal/app -run 'TestBuildScheduleWorkbook|TestMonthScheduleExportData' -count=1
```

Expected: FAIL because `scheduleExportData`, `buildScheduleWorkbook`, and `monthScheduleExportData` are not defined yet.

- [ ] **Step 4: Implement the export data types and store-scoped queries**

In `schedule_export.go`, define focused internal types:

```go
type scheduleExportData struct {
	Store  Store
	Month  string
	Users  []User
	Shifts []ShiftTemplate
	Items  []ScheduleItem
}
```

Implement `monthScheduleExportData` as follows:

1. Call `monthRange(month)` before any query.
2. Query the current store row by `id` and return `notFound("门店不存在")` for no row.
3. Query users with `storeId = ? AND isActive = 1 ORDER BY role ASC, displayName ASC`, scanning only the public user fields needed by the workbook.
4. Query shift templates with `storeId = ? ORDER BY sortOrder ASC, createdAt ASC`.
5. Query schedules for the store and date range, joining users and shift templates on both ID and store ID, and ordering by user ID then work date. Use the same `ScheduleItem` fields and `DBDate` scan behavior as `monthSchedules`.
6. Return an empty, non-nil slice for each collection when no rows exist so workbook loops are deterministic.

Do not reuse an HTTP handler for these queries, and do not accept user IDs, shift IDs, or store IDs from query parameters.

- [ ] **Step 5: Implement workbook generation in memory**

Implement `buildScheduleWorkbook` using `excelize.NewFile`, deleting the default sheet, then creating exactly `月度排班表` and `班次说明` in that order. Generate dates with `monthRange` plus UTC `time.ParseInLocation("2006-01-02", ...)` and `AddDate`.

Use stable helpers in the same file:

```go
func excelColumnName(number int) string
func normalizedExcelColor(raw string) string
func exportFileName(storeName, month string) (safeUTF8Name, asciiFallback string)
func excelRoleLabel(role string) string
```

Workbook requirements:

- Merge `A1:<last-date-column>1`, set the store/month title, and write row 2 headers `员工`, `角色`, then `DD\n周X` date headers.
- Write active users beginning at row 3; map `userId + NUL + workDate` to `ScheduleItem` for O(1) cell lookup.
- Use `-` for absent schedules and shift names for present schedules.
- Apply borders/alignment, title/header styles, weekend fills, and normalized shift colors. For invalid colors use `#909399` and white text.
- Freeze rows 1-2 and columns A-B using the excelize pane API; set landscape/fit-to-width print options where supported, returning an error only for workbook generation failures, not optional page setup calls.
- Add `班次说明` headers `班次名称`, `班次编码`, `开始时间`, `结束时间`, `类型`, `颜色`; write `休息` for off-shift times and `休息`/`上班` for type; fill the color sample cell.
- Set practical column widths and freeze the first legend row.
- Write the workbook to a `bytes.Buffer` with `Write(&buffer)` and return `buffer.Bytes()`.

Use only escaped, controlled cell content. Excel formula injection is not expected from the current names, but prefix any user-controlled value beginning with `=`, `+`, `-`, or `@` with an apostrophe before writing it as text; preserve the literal `-` placeholder by applying the guard only to user-controlled values.

- [ ] **Step 6: Run focused tests and formatting checks**

Run:

```powershell
```

Expected: PASS.

- [ ] **Step 7: Commit the backend library/model unit**

Do not include frontend or route changes in this commit.

```powershell
```

---

### Task 2: Expose the authenticated binary download route

**Files:**
- Modify: `apps/server/internal/app/server.go:86-90`
- Modify: `apps/server/internal/app/schedule_export.go`
- Test: `apps/server/internal/app/server_test.go`

**Interfaces:**
- Consumes: `monthScheduleExportData`, `buildScheduleWorkbook`, `exportFileName`, and `requestClaims` from Task 1.
- Produces: `GET /api/schedules/export?month=YYYY-MM` returning an authenticated Excel attachment.

- [ ] **Step 1: Write failing HTTP integration tests**

Add a test using the existing seeded SQLite helpers:

```go
func TestScheduleExportEndpoint(t *testing.T) {
	// Start New with SeedOnBoot, log in manager and staff after seed.
	// GET /api/schedules/export?month=<month> with each token.
	// Assert 200, Excel content type, attachment disposition, and non-empty body.
	// Open the body through excelize.OpenReader and assert both sheet names.
	// Assert no Authorization returns 401 and month=2026-13 returns 400.
}
```

Add a second assertion that inserts an inactive user and a second-store user/schedule, then opens the generated workbook and checks that neither name appears in the first sheet.

- [ ] **Step 2: Run the endpoint tests and verify the route fails**

Run:

```powershell
```

Expected: FAIL with a 404 because the route is not registered.

- [ ] **Step 3: Register the route with normal authentication**

Add this route next to the existing read-only schedule route:

```go
s.route(mux, "GET /api/schedules/export", s.authenticate(s.exportMonthSchedules))
```

Do not pass `RoleManager`; staff must be allowed.

- [ ] **Step 4: Implement the HTTP handler**

Implement `exportMonthSchedules`:

1. Read `month := r.URL.Query().Get("month")`.
2. Call `monthScheduleExportData(r.Context(), requestClaims(r).StoreID, month)`.
3. Call `buildScheduleWorkbook(data)` before mutating response headers.
4. Build a safe UTF-8 filename and ASCII fallback from the store name and month. Set `Content-Type`, `Content-Disposition` with both `filename` and RFC 5987 `filename*`, `Content-Length`, and `Cache-Control: no-store`.
5. Write status `200` and the bytes with `w.Write`.

Use `url.PathEscape` for the UTF-8 filename parameter and quote the ASCII fallback. Never interpolate raw store names into a header. If data or workbook creation fails, return the error to the existing `route` wrapper before writing headers.

- [ ] **Step 5: Run endpoint tests and the complete Go suite**

Run:

```powershell
```

Expected: all pass.

- [ ] **Step 6: Commit the route unit**

```powershell
```

---

### Task 3: Add the Web Blob download flow and unsaved-draft guard

**Files:**
- Modify: `apps/web/src/api/index.ts`
- Modify: `apps/web/src/api/http.ts`
- Modify: `apps/web/src/views/ScheduleView.vue`
- Test: `apps/web/src/views/ScheduleView.spec.ts` if a test runner exists; otherwise verify with build and focused browser/manual checks.

**Interfaces:**
- Consumes: `GET /api/schedules/export` and `auth.token` through the existing Axios interceptor.
- Produces: `exportMonthSchedulesApi(month: string)` returning `AxiosResponse<Blob>` and a `ScheduleView.vue` `exportMonth` action.

- [ ] **Step 1: Inspect the existing frontend test setup before adding a test dependency**

Run:

```powershell
Test-Path -LiteralPath 'apps/web/vitest.config.ts'
Test-Path -LiteralPath 'apps/web/src/**/*.spec.ts'
```

If no runner/config exists, do not add Vitest solely for this button; keep the verification in the existing build and manual flow. Do not expand frontend scope with a new test framework.

- [ ] **Step 2: Add the Blob API wrapper**

In `apps/web/src/api/index.ts`, add:

```ts
export const exportMonthSchedulesApi = (month: string) =>
  http.get<Blob>('/schedules/export', {
    params: { month },
    responseType: 'blob',
  });
```

In `apps/web/src/api/http.ts`, preserve the current JSON behavior and add a helper/interceptor branch for Blob errors. When `error.response?.data` is a `Blob`, read it with `await blob.text()`, parse `{ message }` if JSON, and reject with the same `Error` shape used by the current interceptor. Do not convert successful Blob responses.

- [ ] **Step 3: Add the export button and state to `ScheduleView.vue`**

Add `Download` from `@element-plus/icons-vue`, import `exportMonthSchedulesApi`, and add `const exporting = ref(false)`.

Render the button for both roles beside refresh:

```vue
<el-button :loading="exporting" :disabled="exporting" @click="exportMonth">
  <el-icon><Download /></el-icon>
  导出 Excel
</el-button>
```

Implement:

```ts
async function exportMonth() {
  if (auth.isManager && Object.keys(drafts).length) {
    ElMessage.warning('请先保存修改后再导出');
    return;
  }
  exporting.value = true;
  try {
    const response = await exportMonthSchedulesApi(monthValue.value);
    const filename = filenameFromDisposition(response.headers['content-disposition'])
      || `${monthValue.value}-排班表.xlsx`;
    const url = URL.createObjectURL(response.data);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
    ElMessage.success('排班表已导出');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    exporting.value = false;
  }
}
```

Implement `filenameFromDisposition` locally in the component or a narrowly scoped API helper. Prefer `filename*` with `decodeURIComponent`, strip path separators/control characters, and reject an empty/unsafe result. Keep the fallback ASCII-safe and append `.xlsx` if the server value does not include it.

- [ ] **Step 4: Run the frontend build and inspect TypeScript output**

Run:

```powershell
npm run build -w @shift/web
```

Expected: Vite build succeeds with no TypeScript errors. If the existing app exposes a dev server, manually verify manager draft blocking, manager download, staff download, month switching, and error toast behavior.

- [ ] **Step 5: Commit the Web unit**

```powershell
```

---

### Task 4: Synchronize documentation and run end-to-end verification

**Files:**
- Modify: `README.md:274-281,523-530`
- Verify: `docs/superpowers/specs/2026-08-13-monthly-schedule-export-design.md`
- Verify: `docs/superpowers/plans/2026-08-13-monthly-schedule-export.md`

**Interfaces:**
- Consumes: the completed backend route and Web button from Tasks 1-3.
- Produces: user-facing API documentation and verified build/test evidence.

- [ ] **Step 1: Update README API and feature lists**

Add `GET /schedules/export?month=YYYY-MM` to the 排班 API table with the description “下载当前门店指定月份的 Excel 排班表（店长和店员均可）”. Remove or update the old “Excel 导出” future-extension bullet so the README does not describe shipped behavior as pending.

- [ ] **Step 2: Run all automated checks**

Run each command separately and record the observed result:

```powershell

Expected: Go tests pass, Web build passes, and the combined project build produces the Go server and Web distribution successfully.

- [ ] **Step 3: Run a live smoke test against the local server**

Start the existing development services with:

```powershell
npm run dev:server
npm run dev:web
```

Use the browser at `http://localhost:5173`, log in with a seeded account, select a month, download the workbook, and open it with `excelize` or an installed spreadsheet viewer. Confirm the two sheets, active employees, dates, shift colors, and legend. Stop any process started for the smoke test after verification.

- [ ] **Step 4: Inspect the final diff and status**

Run:

```powershell
```

Confirm no generated database, build output, temporary workbook, log, secret, or unrelated user change is included.

- [ ] **Step 5: Commit the documentation and final verified changes**

Only after the verification commands pass:

```powershell
```

Do not amend earlier commits; if a check fails, fix it and create a new commit following the repository style.

## Self-Review Checklist

- [ ] Every requirement in the approved design has a task: authenticated route, store isolation, active users, two sheets, colors, weekend styling, freeze/print settings, Blob errors, draft guard, manager/staff access, no mini-program change, and tests.
- [ ] No step relies on an undefined function or type; Task 1 produces the exact data and workbook functions consumed by Task 2.
- [ ] No placeholder instructions such as “TBD”, “TODO”, “implement later”, or “add appropriate handling” remain.
- [ ] Optional Excel page-layout API failures are non-fatal, while workbook creation and response-write failures remain fatal.
- [ ] The plan does not introduce a new frontend test framework when the repository has no existing frontend test setup.

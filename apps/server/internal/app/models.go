package app

import (
	"database/sql"
	"encoding/json"
	"time"
)

const (
	RoleManager = "manager"
	RoleStaff   = "staff"

	RequestLeave  = "leave"
	RequestSwap   = "swap"
	RequestChange = "change"

	RequestPending   = "pending"
	RequestApproved  = "approved"
	RequestRejected  = "rejected"
	RequestCancelled = "cancelled"
)

type User struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"displayName"`
	PasswordHash       string `json:"-"`
	Role               string `json:"role"`
	StoreID            string `json:"storeId"`
	IsActive           bool   `json:"isActive"`
	MustChangePassword bool   `json:"mustChangePassword,omitempty"`
	AuthVersion        int    `json:"-"`
	CreatedAt          DBTime `json:"-"`
	UpdatedAt          DBTime `json:"-"`
}

type Store struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Address   *string `json:"address"`
	CreatedAt DBTime  `json:"createdAt"`
	UpdatedAt DBTime  `json:"updatedAt"`
}

type ShiftTemplate struct {
	ID        string `json:"id"`
	StoreID   string `json:"storeId"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
	IsOff     bool   `json:"isOff"`
	CreatedAt DBTime `json:"createdAt"`
	UpdatedAt DBTime `json:"updatedAt"`
}

type Schedule struct {
	ID              string
	StoreID         string
	UserID          string
	WorkDate        string
	ShiftTemplateID string
	Status          string
	Version         int
	Note            sql.NullString
	CreatedByID     sql.NullString
	UpdatedByID     sql.NullString
	CreatedAt       DBTime
	UpdatedAt       DBTime
}

type ScheduleItem struct {
	ID              string  `json:"id"`
	UserID          string  `json:"userId"`
	UserName        string  `json:"userName"`
	WorkDate        string  `json:"workDate"`
	ShiftTemplateID string  `json:"shiftTemplateId"`
	ShiftName       string  `json:"shiftName"`
	ShiftColor      string  `json:"shiftColor"`
	Status          string  `json:"status"`
	Version         int     `json:"version"`
	Note            *string `json:"note"`
}

type ScheduleRequest struct {
	ID           string  `json:"id"`
	StoreID      string  `json:"storeId"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	RequesterID  string  `json:"requesterId"`
	Requester    User    `json:"requester"`
	TargetUserID *string `json:"targetUserId"`
	TargetUser   *User   `json:"targetUser"`
	FromDate     string  `json:"fromDate"`
	ToDate       *string `json:"toDate"`
	FromShiftID  *string `json:"fromShiftId"`
	ToShiftID    *string `json:"toShiftId"`
	Reason       *string `json:"reason"`
	ReviewNote   *string `json:"reviewNote"`
	ApproverID   *string `json:"approverId"`
	ReviewedAt   *DBTime `json:"reviewedAt"`
	CreatedAt    DBTime  `json:"createdAt"`
	UpdatedAt    DBTime  `json:"updatedAt"`
}

type Notification struct {
	ID          string  `json:"id"`
	StoreID     string  `json:"storeId"`
	UserID      string  `json:"userId"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	IsRead      bool    `json:"isRead"`
	RelatedType *string `json:"relatedType"`
	RelatedID   *string `json:"relatedId"`
	CreatedAt   DBTime  `json:"createdAt"`
}

type DBTime struct {
	time.Time
}

type DBDate string

type NullDBDate struct {
	DBDate
	Valid bool
}

func (d *NullDBDate) Scan(value any) error {
	if value == nil {
		d.DBDate = ""
		d.Valid = false
		return nil
	}
	d.Valid = true
	return d.DBDate.Scan(value)
}

func (d *DBDate) Scan(value any) error {
	if value == nil {
		*d = ""
		return nil
	}
	if parsed, ok := value.(time.Time); ok {
		*d = DBDate(parsed.Format("2006-01-02"))
		return nil
	}
	var raw string
	switch value := value.(type) {
	case string:
		raw = value
	case []byte:
		raw = string(value)
	default:
		raw = ""
	}
	if len(raw) >= len("2006-01-02") {
		if parsed, err := time.Parse("2006-01-02", raw[:len("2006-01-02")]); err == nil {
			*d = DBDate(parsed.Format("2006-01-02"))
			return nil
		}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		*d = DBDate(parsed.Format("2006-01-02"))
		return nil
	}
	return &time.ParseError{Value: raw, Layout: "database date"}
}

func (t *DBTime) Scan(value any) error {
	if value == nil {
		t.Time = time.Time{}
		return nil
	}
	if parsed, ok := value.(time.Time); ok {
		t.Time = parsed.UTC()
		return nil
	}
	var raw string
	switch value := value.(type) {
	case string:
		raw = value
	case []byte:
		raw = string(value)
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		"2006-01-02",
	} {
		if parsed, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			t.Time = parsed.UTC()
			return nil
		}
	}
	return &time.ParseError{Value: raw, Layout: "database datetime"}
}

func (t DBTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.UTC().Format(time.RFC3339Nano))
}

func ptrFromNull(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

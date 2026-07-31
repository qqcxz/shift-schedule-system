package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultManagerUsername = "qqcxz"
	defaultManagerPassword = "123456"
	legacyManagerUsername  = "manager"
)

func seedIfEmpty(ctx context.Context, db *sql.DB, cfg Config) error {
	managerUsername := cfg.SeedManagerUsername
	if managerUsername == "" {
		managerUsername = defaultManagerUsername
	}
	managerPassword := cfg.SeedManagerPassword
	if managerPassword == "" {
		managerPassword = defaultManagerPassword
	}
	staffPassword := cfg.SeedStaffPassword
	if staffPassword == "" {
		staffPassword = "123456"
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `users`").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return migrateLegacyManager(ctx, db, managerUsername, managerPassword)
	}

	managerHash, err := bcrypt.GenerateFromPassword([]byte(managerPassword), 10)
	if err != nil {
		return err
	}
	staffHash, err := bcrypt.GenerateFromPassword([]byte(staffPassword), 10)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := nowDB()
	storeID := uuid.NewString()
	managerID := uuid.NewString()
	if _, err := tx.ExecContext(ctx, "INSERT INTO `stores` (`id`, `name`, `address`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?)", storeID, "示例门店", "示例路 1 号", now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO `users` (`id`, `username`, `displayName`, `passwordHash`, `role`, `storeId`, `isActive`, `authVersion`, `mustChangePassword`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?)", managerID, managerUsername, "店长", string(managerHash), RoleManager, storeID, true, now, now); err != nil {
		return err
	}

	staffDefs := []struct{ username, displayName string }{
		{"staff1", "店员小李"},
		{"staff2", "店员小张"},
		{"staff3", "店员小陈"},
	}
	staffIDs := make([]string, 0, len(staffDefs))
	for _, definition := range staffDefs {
		id := uuid.NewString()
		staffIDs = append(staffIDs, id)
		if _, err := tx.ExecContext(ctx, "INSERT INTO `users` (`id`, `username`, `displayName`, `passwordHash`, `role`, `storeId`, `isActive`, `authVersion`, `mustChangePassword`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, 1, 0, ?, ?)", id, definition.username, definition.displayName, string(staffHash), RoleStaff, storeID, true, now, now); err != nil {
			return err
		}
	}

	shiftDefs := []struct {
		name, code, start, end, color string
		off                           bool
	}{
		{"早班", "MORNING", "09:00", "13:00", "#67C23A", false},
		{"中班", "NOON", "13:00", "17:00", "#E6A23C", false},
		{"晚班", "NIGHT", "17:00", "21:00", "#409EFF", false},
		{"休息", "OFF", "00:00", "00:00", "#909399", true},
	}
	shiftIDs := make([]string, 0, len(shiftDefs))
	for index, definition := range shiftDefs {
		id := uuid.NewString()
		shiftIDs = append(shiftIDs, id)
		if _, err := tx.ExecContext(ctx, "INSERT INTO `shift_templates` (`id`, `storeId`, `name`, `code`, `startTime`, `endTime`, `color`, `sortOrder`, `isOff`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", id, storeID, definition.name, definition.code, definition.start, definition.end, definition.color, index+1, definition.off, now, now); err != nil {
			return err
		}
	}

	localNow := time.Now()
	daysInMonth := time.Date(localNow.Year(), localNow.Month()+1, 0, 0, 0, 0, 0, localNow.Location()).Day()
	for day := 1; day <= daysInMonth; day++ {
		workDate := fmt.Sprintf("%04d-%02d-%02d", localNow.Year(), int(localNow.Month()), day)
		for index, userID := range staffIDs {
			isOff := (day+index)%7 == 0
			shiftID := shiftIDs[(day+index)%3]
			status := "normal"
			if isOff {
				shiftID = shiftIDs[3]
				status = "leave"
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO `schedules` (`id`, `storeId`, `userId`, `workDate`, `shiftTemplateId`, `status`, `version`, `createdById`, `updatedById`, `createdAt`, `updatedAt`) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)", uuid.NewString(), storeID, userID, workDate, shiftID, status, managerID, managerID, now, now); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("seed data ready: manager=%s", managerUsername)
	return nil
}

func migrateLegacyManager(ctx context.Context, db *sql.DB, managerUsername, managerPassword string) error {
	current, err := findUserByUsername(ctx, db, managerUsername)
	if err == nil {
		initialPasswords := []string{defaultManagerPassword, "200395ljf", managerPassword}
		needsPasswordChange := false
		for _, initialPassword := range initialPasswords {
			if bcrypt.CompareHashAndPassword([]byte(current.PasswordHash), []byte(initialPassword)) == nil {
				needsPasswordChange = true
				break
			}
		}
		if needsPasswordChange {
			hash, hashErr := bcrypt.GenerateFromPassword([]byte(managerPassword), 10)
			if hashErr != nil {
				return hashErr
			}
			_, err = db.ExecContext(ctx, "UPDATE `users` SET `passwordHash` = ?, `mustChangePassword` = 1, `authVersion` = `authVersion` + 1, `updatedAt` = ? WHERE `id` = ?", string(hash), nowDB(), current.ID)
		}
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	legacy, err := findUserByUsername(ctx, db, legacyManagerUsername)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(managerPassword), 10)
	if err != nil {
		return err
	}
	displayName := legacy.DisplayName
	if displayName == "" {
		displayName = "店长"
	}
	_, err = db.ExecContext(ctx, "UPDATE `users` SET `username` = ?, `displayName` = ?, `role` = ?, `isActive` = 1, `passwordHash` = ?, `mustChangePassword` = 1, `authVersion` = `authVersion` + 1, `updatedAt` = ? WHERE `id` = ?", managerUsername, displayName, RoleManager, string(hash), nowDB(), legacy.ID)
	return err
}

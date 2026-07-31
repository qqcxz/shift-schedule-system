package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

func openDatabase(ctx context.Context, cfg Config) (*sql.DB, error) {
	var driver, dsn string
	if cfg.DBType == "mysql" {
		driver = "mysql"
		dsn = mysqlDSN(cfg)
	} else {
		driver = "sqlite"
		absolute, err := filepath.Abs(cfg.DBDatabase)
		if err != nil {
			return nil, fmt.Errorf("resolve DB_DATABASE: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
		dsn = "file:" + filepath.ToSlash(absolute) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if cfg.DBType == "sqlite" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(30 * time.Minute)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := migrate(ctx, db, cfg.DBType); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func mysqlDSN(cfg Config) string {
	// NewConfig preserves the driver's defaults. A zero-value Config formats
	// allowNativePasswords=false and rejects existing BaoTa/MySQL users that
	// authenticate with mysql_native_password.
	mysqlConfig := mysqlDriver.NewConfig()
	mysqlConfig.User = cfg.DBUser
	mysqlConfig.Passwd = cfg.DBPassword
	mysqlConfig.Net = "tcp"
	mysqlConfig.Addr = fmt.Sprintf("%s:%d", cfg.DBHost, cfg.DBPort)
	mysqlConfig.DBName = cfg.DBName
	mysqlConfig.Collation = "utf8mb4_unicode_ci"
	mysqlConfig.ParseTime = true
	mysqlConfig.Timeout = 10 * time.Second
	mysqlConfig.ReadTimeout = 30 * time.Second
	mysqlConfig.WriteTimeout = 30 * time.Second
	mysqlConfig.Params = map[string]string{"charset": "utf8mb4"}
	return mysqlConfig.FormatDSN()
}

func migrate(ctx context.Context, db *sql.DB, dialect string) error {
	statements := sqliteSchema
	if dialect == "mysql" {
		statements = mysqlSchema
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply database migration: %w", err)
		}
	}
	if err := ensureUserSecurityColumns(ctx, db, dialect); err != nil {
		return err
	}
	if err := recordMigration(ctx, db, dialect, 1); err != nil {
		return err
	}
	if err := recordMigration(ctx, db, dialect, 2); err != nil {
		return err
	}
	return nil
}

func recordMigration(ctx context.Context, db *sql.DB, dialect string, version int) error {
	query := "INSERT OR IGNORE INTO `schema_migrations` (`version`, `appliedAt`) VALUES (?, ?)"
	if dialect == "mysql" {
		query = "INSERT IGNORE INTO `schema_migrations` (`version`, `appliedAt`) VALUES (?, ?)"
	}
	if _, err := db.ExecContext(ctx, query, version, nowDB()); err != nil {
		return fmt.Errorf("record database migration %d: %w", version, err)
	}
	return nil
}

func ensureUserSecurityColumns(ctx context.Context, db *sql.DB, dialect string) error {
	columns := []struct {
		name       string
		definition string
	}{
		{name: "authVersion", definition: "integer NOT NULL DEFAULT 1"},
		{name: "mustChangePassword", definition: "boolean NOT NULL DEFAULT 0"},
	}
	for _, column := range columns {
		query := "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = ?"
		if dialect == "mysql" {
			query = "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'users' AND column_name = ?"
		}
		var count int
		if err := db.QueryRowContext(ctx, query, column.name).Scan(&count); err != nil {
			return fmt.Errorf("check %s column: %w", column.name, err)
		}
		if count > 0 {
			continue
		}
		if _, err := db.ExecContext(ctx, "ALTER TABLE `users` ADD COLUMN `"+column.name+"` "+column.definition); err != nil {
			return fmt.Errorf("add %s column: %w", column.name, err)
		}
	}
	return nil
}

var sqliteSchema = splitSQL(`
CREATE TABLE IF NOT EXISTS "schema_migrations" ("version" integer PRIMARY KEY NOT NULL, "appliedAt" datetime NOT NULL);
CREATE TABLE IF NOT EXISTS "stores" ("id" varchar PRIMARY KEY NOT NULL, "name" varchar NOT NULL, "address" varchar, "createdAt" datetime NOT NULL, "updatedAt" datetime NOT NULL);
CREATE TABLE IF NOT EXISTS "users" ("id" varchar PRIMARY KEY NOT NULL, "username" varchar NOT NULL UNIQUE, "displayName" varchar NOT NULL, "passwordHash" varchar NOT NULL, "role" varchar NOT NULL DEFAULT 'staff', "storeId" varchar NOT NULL, "isActive" boolean NOT NULL DEFAULT 1, "authVersion" integer NOT NULL DEFAULT 1, "mustChangePassword" boolean NOT NULL DEFAULT 0, "createdAt" datetime NOT NULL, "updatedAt" datetime NOT NULL, FOREIGN KEY ("storeId") REFERENCES "stores" ("id") ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS "shift_templates" ("id" varchar PRIMARY KEY NOT NULL, "storeId" varchar NOT NULL, "name" varchar NOT NULL, "code" varchar NOT NULL, "startTime" varchar NOT NULL, "endTime" varchar NOT NULL, "color" varchar NOT NULL DEFAULT '#409EFF', "sortOrder" integer NOT NULL DEFAULT 0, "isOff" boolean NOT NULL DEFAULT 0, "createdAt" datetime NOT NULL, "updatedAt" datetime NOT NULL);
CREATE TABLE IF NOT EXISTS "schedules" ("id" varchar PRIMARY KEY NOT NULL, "storeId" varchar NOT NULL, "userId" varchar NOT NULL, "workDate" date NOT NULL, "shiftTemplateId" varchar NOT NULL, "status" varchar NOT NULL DEFAULT 'normal', "version" integer NOT NULL DEFAULT 1, "note" varchar, "createdById" varchar, "updatedById" varchar, "createdAt" datetime NOT NULL, "updatedAt" datetime NOT NULL, FOREIGN KEY ("userId") REFERENCES "users" ("id") ON DELETE CASCADE, FOREIGN KEY ("shiftTemplateId") REFERENCES "shift_templates" ("id") ON DELETE RESTRICT);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_35c4d9beec50a2e9dc7fc89012" ON "schedules" ("storeId", "userId", "workDate");
CREATE TABLE IF NOT EXISTS "schedule_requests" ("id" varchar PRIMARY KEY NOT NULL, "storeId" varchar NOT NULL, "type" varchar NOT NULL, "status" varchar NOT NULL DEFAULT 'pending', "requesterId" varchar NOT NULL, "targetUserId" varchar, "fromDate" date NOT NULL, "toDate" date, "fromShiftId" varchar, "toShiftId" varchar, "reason" text, "reviewNote" text, "approverId" varchar, "reviewedAt" datetime, "createdAt" datetime NOT NULL, "updatedAt" datetime NOT NULL, FOREIGN KEY ("requesterId") REFERENCES "users" ("id") ON DELETE CASCADE, FOREIGN KEY ("targetUserId") REFERENCES "users" ("id") ON DELETE SET NULL);
CREATE TABLE IF NOT EXISTS "notifications" ("id" varchar PRIMARY KEY NOT NULL, "storeId" varchar NOT NULL, "userId" varchar NOT NULL, "title" varchar NOT NULL, "content" text NOT NULL, "isRead" boolean NOT NULL DEFAULT 0, "relatedType" varchar, "relatedId" varchar, "createdAt" datetime NOT NULL);
`)

var mysqlSchema = splitSQL(`
CREATE TABLE IF NOT EXISTS ` + "`schema_migrations`" + ` (` + "`version`" + ` int NOT NULL, ` + "`appliedAt`" + ` datetime(6) NOT NULL, PRIMARY KEY (` + "`version`" + `)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS ` + "`stores`" + ` (` + "`id`" + ` varchar(36) NOT NULL, ` + "`name`" + ` varchar(255) NOT NULL, ` + "`address`" + ` varchar(255) NULL, ` + "`createdAt`" + ` datetime(6) NOT NULL, ` + "`updatedAt`" + ` datetime(6) NOT NULL, PRIMARY KEY (` + "`id`" + `)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS ` + "`users`" + ` (` + "`id`" + ` varchar(36) NOT NULL, ` + "`username`" + ` varchar(255) NOT NULL, ` + "`displayName`" + ` varchar(255) NOT NULL, ` + "`passwordHash`" + ` varchar(255) NOT NULL, ` + "`role`" + ` varchar(255) NOT NULL DEFAULT 'staff', ` + "`storeId`" + ` varchar(36) NOT NULL, ` + "`isActive`" + ` tinyint NOT NULL DEFAULT 1, ` + "`authVersion`" + ` int NOT NULL DEFAULT 1, ` + "`mustChangePassword`" + ` tinyint NOT NULL DEFAULT 0, ` + "`createdAt`" + ` datetime(6) NOT NULL, ` + "`updatedAt`" + ` datetime(6) NOT NULL, PRIMARY KEY (` + "`id`" + `), UNIQUE KEY ` + "`UQ_fe0bb3f6520ee0469504521e710`" + ` (` + "`username`" + `), CONSTRAINT ` + "`FK_c82cd4fa8f0ac4a74328abe997a`" + ` FOREIGN KEY (` + "`storeId`" + `) REFERENCES ` + "`stores`" + ` (` + "`id`" + `) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS ` + "`shift_templates`" + ` (` + "`id`" + ` varchar(36) NOT NULL, ` + "`storeId`" + ` varchar(36) NOT NULL, ` + "`name`" + ` varchar(255) NOT NULL, ` + "`code`" + ` varchar(255) NOT NULL, ` + "`startTime`" + ` varchar(255) NOT NULL, ` + "`endTime`" + ` varchar(255) NOT NULL, ` + "`color`" + ` varchar(255) NOT NULL DEFAULT '#409EFF', ` + "`sortOrder`" + ` int NOT NULL DEFAULT 0, ` + "`isOff`" + ` tinyint NOT NULL DEFAULT 0, ` + "`createdAt`" + ` datetime(6) NOT NULL, ` + "`updatedAt`" + ` datetime(6) NOT NULL, PRIMARY KEY (` + "`id`" + `)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS ` + "`schedules`" + ` (` + "`id`" + ` varchar(36) NOT NULL, ` + "`storeId`" + ` varchar(36) NOT NULL, ` + "`userId`" + ` varchar(36) NOT NULL, ` + "`workDate`" + ` date NOT NULL, ` + "`shiftTemplateId`" + ` varchar(36) NOT NULL, ` + "`status`" + ` varchar(255) NOT NULL DEFAULT 'normal', ` + "`version`" + ` int NOT NULL DEFAULT 1, ` + "`note`" + ` varchar(255) NULL, ` + "`createdById`" + ` varchar(36) NULL, ` + "`updatedById`" + ` varchar(36) NULL, ` + "`createdAt`" + ` datetime(6) NOT NULL, ` + "`updatedAt`" + ` datetime(6) NOT NULL, PRIMARY KEY (` + "`id`" + `), UNIQUE KEY ` + "`IDX_35c4d9beec50a2e9dc7fc89012`" + ` (` + "`storeId`" + `, ` + "`userId`" + `, ` + "`workDate`" + `), CONSTRAINT ` + "`FK_19c54f24597b318be3892114c75`" + ` FOREIGN KEY (` + "`userId`" + `) REFERENCES ` + "`users`" + ` (` + "`id`" + `) ON DELETE CASCADE, CONSTRAINT ` + "`FK_c71ca46fb803b31eb8d0dbbf9f7`" + ` FOREIGN KEY (` + "`shiftTemplateId`" + `) REFERENCES ` + "`shift_templates`" + ` (` + "`id`" + `) ON DELETE RESTRICT) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS ` + "`schedule_requests`" + ` (` + "`id`" + ` varchar(36) NOT NULL, ` + "`storeId`" + ` varchar(36) NOT NULL, ` + "`type`" + ` varchar(255) NOT NULL, ` + "`status`" + ` varchar(255) NOT NULL DEFAULT 'pending', ` + "`requesterId`" + ` varchar(36) NOT NULL, ` + "`targetUserId`" + ` varchar(36) NULL, ` + "`fromDate`" + ` date NOT NULL, ` + "`toDate`" + ` date NULL, ` + "`fromShiftId`" + ` varchar(36) NULL, ` + "`toShiftId`" + ` varchar(36) NULL, ` + "`reason`" + ` text NULL, ` + "`reviewNote`" + ` text NULL, ` + "`approverId`" + ` varchar(36) NULL, ` + "`reviewedAt`" + ` datetime(6) NULL, ` + "`createdAt`" + ` datetime(6) NOT NULL, ` + "`updatedAt`" + ` datetime(6) NOT NULL, PRIMARY KEY (` + "`id`" + `), CONSTRAINT ` + "`FK_3c76a881e407dd8b58398a8ad76`" + ` FOREIGN KEY (` + "`requesterId`" + `) REFERENCES ` + "`users`" + ` (` + "`id`" + `) ON DELETE CASCADE, CONSTRAINT ` + "`FK_641b364374a65f406ca662c16ff`" + ` FOREIGN KEY (` + "`targetUserId`" + `) REFERENCES ` + "`users`" + ` (` + "`id`" + `) ON DELETE SET NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS ` + "`notifications`" + ` (` + "`id`" + ` varchar(36) NOT NULL, ` + "`storeId`" + ` varchar(36) NOT NULL, ` + "`userId`" + ` varchar(36) NOT NULL, ` + "`title`" + ` varchar(255) NOT NULL, ` + "`content`" + ` text NOT NULL, ` + "`isRead`" + ` tinyint NOT NULL DEFAULT 0, ` + "`relatedType`" + ` varchar(255) NULL, ` + "`relatedId`" + ` varchar(36) NULL, ` + "`createdAt`" + ` datetime(6) NOT NULL, PRIMARY KEY (` + "`id`" + `)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`)

func splitSQL(raw string) []string {
	parts := strings.Split(raw, ";")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if statement := strings.TrimSpace(part); statement != "" {
			result = append(result, statement)
		}
	}
	return result
}

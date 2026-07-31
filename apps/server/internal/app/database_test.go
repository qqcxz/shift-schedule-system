package app

import (
	"strings"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestMySQLDSNAllowsNativePasswordUsers(t *testing.T) {
	dsn := mysqlDSN(Config{
		DBHost:     "127.0.0.1",
		DBPort:     3306,
		DBUser:     "shift",
		DBPassword: "password",
		DBName:     "shift_schedule",
	})
	if strings.Contains(dsn, "allowNativePasswords=false") {
		t.Fatalf("DSN must not disable native password authentication: %s", dsn)
	}
	parsed, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	if !parsed.AllowNativePasswords {
		t.Fatal("native password authentication should be enabled for existing MySQL users")
	}
}

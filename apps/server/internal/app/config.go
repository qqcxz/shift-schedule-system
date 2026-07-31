package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                int
	DBType              string
	DBDatabase          string
	DBHost              string
	DBPort              int
	DBUser              string
	DBPassword          string
	DBName              string
	JWTSecret           string
	CORSOrigins         []string
	SeedOnBoot          bool
	SeedManagerUsername string
	SeedManagerPassword string
	SeedStaffPassword   string
	ScheduleKeepMonths  int
	WebDist             string
}

func LoadConfig() (Config, error) {
	loadDotEnv()
	port, err := envInt("PORT", 3000)
	if err != nil {
		return Config{}, err
	}
	dbPort, err := envInt("DB_PORT", 3306)
	if err != nil {
		return Config{}, err
	}
	keepMonths, err := envInt("SCHEDULE_KEEP_MONTHS", 12)
	if err != nil {
		return Config{}, err
	}
	if port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535")
	}
	if dbPort < 1 || dbPort > 65535 {
		return Config{}, fmt.Errorf("DB_PORT must be between 1 and 65535")
	}
	if keepMonths < 1 || keepMonths > 120 {
		return Config{}, fmt.Errorf("SCHEDULE_KEEP_MONTHS must be between 1 and 120")
	}

	dbType := strings.ToLower(strings.TrimSpace(env("DB_TYPE", "sqljs")))
	switch dbType {
	case "sqljs", "sqlite", "sqlite3":
		dbType = "sqlite"
	case "mysql":
	default:
		return Config{}, fmt.Errorf("unsupported DB_TYPE %q (use sqljs, sqlite, or mysql)", dbType)
	}
	seedOnBoot, err := envBool("SEED_ON_BOOT", true)
	if err != nil {
		return Config{}, err
	}

	jwtSecret, jwtSecretProvided := configuredSecret("JWT_SECRET", "dev-secret-change-me")
	if dbType == "mysql" && (!jwtSecretProvided || len([]byte(jwtSecret)) < 32) {
		return Config{}, fmt.Errorf("JWT_SECRET must be explicitly set to at least 32 bytes when DB_TYPE=mysql")
	}
	seedManagerUsername := env("SEED_MANAGER_USERNAME", defaultManagerUsername)
	seedManagerPassword, _ := configuredSecret("SEED_MANAGER_PASSWORD", defaultManagerPassword)
	seedStaffPassword, _ := configuredSecret("SEED_STAFF_PASSWORD", "123456")
	if seedOnBoot && (len([]rune(seedManagerPassword)) < 6 || len([]rune(seedStaffPassword)) < 6) {
		return Config{}, fmt.Errorf("SEED_MANAGER_PASSWORD and SEED_STAFF_PASSWORD must be at least 6 characters when seeding")
	}

	return Config{
		Port:                port,
		DBType:              dbType,
		DBDatabase:          env("DB_DATABASE", defaultSQLiteDatabase()),
		DBHost:              env("DB_HOST", "localhost"),
		DBPort:              dbPort,
		DBUser:              env("DB_USER", "shift"),
		DBPassword:          env("DB_PASSWORD", "shift123"),
		DBName:              env("DB_NAME", "shift_schedule"),
		JWTSecret:           jwtSecret,
		CORSOrigins:         parseOrigins(env("CORS_ORIGIN", "http://localhost:5173,http://127.0.0.1:5173,http://localhost,http://127.0.0.1")),
		SeedOnBoot:          seedOnBoot,
		SeedManagerUsername: seedManagerUsername,
		SeedManagerPassword: seedManagerPassword,
		SeedStaffPassword:   seedStaffPassword,
		ScheduleKeepMonths:  keepMonths,
		WebDist:             resolveWebDist(os.Getenv("WEB_DIST")),
	}, nil
}

func defaultSQLiteDatabase() string {
	// Keep the new /app/data location while recognizing the old Node image's
	// /app/apps/server/data path when an existing SQLite file is mounted there.
	for _, candidate := range []string{"./data/shift.sqlite", "./apps/server/data/shift.sqlite"} {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "./data/shift.sqlite"
}

func loadDotEnv() {
	for _, candidate := range []string{".env", filepath.Join("apps", "server", ".env")} {
		file, err := os.Open(candidate)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if key != "" {
				if _, exists := os.LookupEnv(key); !exists {
					_ = os.Setenv(key, value)
				}
			}
		}
		_ = file.Close()
	}
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	raw := env(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

func configuredSecret(key, fallback string) (string, bool) {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value), true
	}
	return fallback, false
}

func envBool(key string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false: %w", key, err)
	}
	return value, nil
}

func parseOrigins(raw string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, 4)
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" && !seen[origin] {
			seen[origin] = true
			result = append(result, origin)
		}
	}
	return result
}

func resolveWebDist(configured string) string {
	candidates := []string{
		configured,
		filepath.Join("..", "web", "dist"),
		filepath.Join("apps", "web", "dist"),
		"public",
		filepath.Join("..", "..", "public"),
		"/app/public",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(filepath.Join(absolute, "index.html")); err == nil && !info.IsDir() {
			return absolute
		}
	}
	return ""
}

func nowDB() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05.000")
}

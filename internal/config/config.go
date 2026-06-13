package config

import (
	"bufio"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"touchline/internal/types"
)

const (
	defaultMockDir         = "mock"
	defaultProvider        = "espn"
	defaultRefreshInterval = 30 * time.Second
	defaultCacheTTL        = 25 * time.Second
	defaultSSHAddress      = "localhost:23234"
	defaultSSHHostKeyPath  = ".ssh/touchline_ed25519"
	defaultTimezone        = "America/New_York"
)

type Config struct {
	Provider        string
	Competition     types.CompetitionCode
	MockDir         string
	ESPNBaseURL     string
	RefreshInterval time.Duration
	CacheTTL        time.Duration
	LogLevel        slog.Level
	// Timezone is the IANA name used to group matches by day and format kickoff
	// times, so a UTC host still shows the correct match day.
	Timezone string

	// SSHEnabled serves the TUI over SSH instead of running it locally.
	SSHEnabled     bool
	SSHAddress     string
	SSHHostKeyPath string
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	return Config{
		Provider:        getEnv("TOUCHLINE_PROVIDER", defaultProvider),
		Competition:     types.CompetitionCode(getEnv("TOUCHLINE_COMPETITION", string(types.CompetitionWorldCup))),
		MockDir:         getEnv("TOUCHLINE_MOCK_DIR", defaultMockDir),
		ESPNBaseURL:     getEnv("TOUCHLINE_ESPN_BASE_URL", ""),
		RefreshInterval: getDurationEnv("TOUCHLINE_REFRESH_INTERVAL", defaultRefreshInterval),
		CacheTTL:        getDurationEnv("TOUCHLINE_CACHE_TTL", defaultCacheTTL),
		LogLevel:        getLogLevelEnv("TOUCHLINE_LOG_LEVEL", slog.LevelInfo),
		Timezone:        getEnv("TOUCHLINE_TIMEZONE", defaultTimezone),

		SSHEnabled:     getBoolEnv("TOUCHLINE_SSH", false),
		SSHAddress:     getEnv("TOUCHLINE_SSH_ADDR", defaultSSHAddress),
		SSHHostKeyPath: getEnv("TOUCHLINE_SSH_HOST_KEY_PATH", defaultSSHHostKeyPath),
	}, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

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

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			_ = os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}

	return fallback
}

func getLogLevelEnv(key string, fallback slog.Level) slog.Level {
	switch strings.ToLower(os.Getenv(key)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		return slog.LevelInfo
	default:
		return fallback
	}
}

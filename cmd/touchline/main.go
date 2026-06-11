package main

import (
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"touchline/internal/api"
	"touchline/internal/cache"
	"touchline/internal/config"
	"touchline/internal/services"
	"touchline/internal/tui"
	"touchline/internal/types"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	provider, err := buildProvider(cfg)
	if err != nil {
		logger.Error("failed to configure provider", "error", err)
		os.Exit(1)
	}

	matchService := services.NewMatchService(
		provider,
		cfg.Competition,
		cache.New[types.Scoreboard](cfg.CacheTTL),
		logger,
	)
	standingService := services.NewStandingService(
		provider,
		cfg.Competition,
		cache.New[[]types.GroupStanding](cfg.CacheTTL),
		logger,
	)

	model := tui.NewModel(matchService, standingService, cfg.RefreshInterval)
	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		logger.Error("touchline exited with error", "error", err)
		os.Exit(1)
	}
}

// buildProvider wires the live data source. ESPN drives matches, details, and
// standings; the mock provider serves bundled JSON for offline use.
func buildProvider(cfg config.Config) (api.FootballProvider, error) {
	switch cfg.Provider {
	case "espn":
		return api.NewESPNProvider(cfg.ESPNBaseURL), nil
	case "mock":
		return api.NewMockProvider(cfg.MockDir), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

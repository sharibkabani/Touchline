package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/muesli/termenv"

	"touchline/internal/config"
	"touchline/internal/services"
	"touchline/internal/tui"
)

// serveSSH exposes the Touchline TUI over SSH using Wish. Every connection gets
// its own Bubble Tea program (and therefore its own view state), while the data
// services and caches are shared across all sessions.
func serveSSH(
	cfg config.Config,
	matchService *services.MatchService,
	standingService *services.StandingService,
	logger *slog.Logger,
) error {
	// The TUI styles use lipgloss' global renderer, which would otherwise detect
	// the (non-interactive) server stdout and strip colors. Forcing a profile
	// keeps the app colorful for connected SSH clients.
	lipgloss.SetColorProfile(termenv.TrueColor)

	handler := func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
		model := tui.NewModel(matchService, standingService, cfg.RefreshInterval)
		return model, []tea.ProgramOption{tea.WithAltScreen()}
	}

	server, err := wish.NewServer(
		wish.WithAddress(cfg.SSHAddress),
		wish.WithHostKeyPath(cfg.SSHHostKeyPath),
		wish.WithMiddleware(
			bubbletea.Middleware(handler),
			activeterm.Middleware(), // require an interactive terminal
			logging.Middleware(),
		),
	)
	if err != nil {
		return err
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("starting touchline ssh server", "address", cfg.SSHAddress)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			logger.Error("ssh server error", "error", err)
			done <- syscall.SIGTERM
		}
	}()

	<-done
	logger.Info("stopping touchline ssh server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		return err
	}
	return nil
}

package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"touchline/internal/types"
)

const requestTimeout = 10 * time.Second

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Leave room for the ASCII banner, subtitle, status line, spacers, and
		// footer so the scroll viewport's paging matches what is on screen.
		m.viewport.Width = max(0, msg.Width-4)
		m.viewport.Height = max(1, msg.Height-13)
		m.ensureSelectedVisible()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			m.err = nil
			return m, m.loadCurrentView(true)
		case "tab":
			m.nextTopLevelView()
			m.err = nil
			m.ensureSelectedVisible()
		case "left", "h":
			if m.currentView == ViewLiveMatches {
				m.shiftSelectedDate(-1)
				m.loading = true
				m.err = nil
				return m, m.loadDashboard(false)
			}
			m.viewport, cmd = m.viewport.Update(msg)
		case "right", "l":
			if m.currentView == ViewLiveMatches {
				m.shiftSelectedDate(1)
				m.loading = true
				m.err = nil
				return m, m.loadDashboard(false)
			}
			m.viewport, cmd = m.viewport.Update(msg)
		case "up", "k":
			if m.currentView == ViewLiveMatches {
				previousID := m.selectedMatchID
				m.moveSelection(-1)
				m.ensureSelectedVisible()
				if match, ok := m.selectedMatch(); ok && match.ID != previousID {
					m.selectedMatchID = match.ID
					m.loading = true
					return m, m.loadMatchDetails(match.ID, false)
				}
			} else {
				m.viewport, cmd = m.viewport.Update(msg)
			}
		case "down", "j":
			if m.currentView == ViewLiveMatches {
				previousID := m.selectedMatchID
				m.moveSelection(1)
				m.ensureSelectedVisible()
				if match, ok := m.selectedMatch(); ok && match.ID != previousID {
					m.selectedMatchID = match.ID
					m.loading = true
					return m, m.loadMatchDetails(match.ID, false)
				}
			} else {
				m.viewport, cmd = m.viewport.Update(msg)
			}
		default:
			m.viewport, cmd = m.viewport.Update(msg)
		}

	case dataLoadedMsg:
		m.loading = false
		if msg.matchesOK {
			m.matches = msg.matches
			m.clampSelection()
		}
		if msg.standingsOK {
			m.standings = msg.standings
		}
		m.err = msg.err
		if !msg.loadedAt.IsZero() {
			m.lastUpdated = msg.loadedAt
		}
		m.ensureSelectedVisible()
		if match, ok := m.selectedMatch(); ok {
			// Always refresh the selected match's details so a live score and
			// timeline keep updating on each auto-refresh. The scoreboard cache
			// was just refreshed, so this reads fresh data without a new request.
			firstLoad := m.selectedMatchID != match.ID || m.details.Match.ID == ""
			m.selectedMatchID = match.ID
			if firstLoad {
				m.loading = true
			}
			return m, m.loadMatchDetails(match.ID, false)
		}
		m.selectedMatchID = ""
		m.details = types.MatchDetails{}

	case detailsLoadedMsg:
		m.loading = false
		if msg.err == nil && (m.selectedMatchID == "" || msg.details.Match.ID == m.selectedMatchID) {
			// Only jump the scroll back to the top when a different match is
			// shown, so background auto-refreshes don't disturb the user.
			matchChanged := m.details.Match.ID != msg.details.Match.ID
			m.details = msg.details
			if matchChanged {
				m.viewport.GotoTop()
			}
		}
		m.err = msg.err
		if !msg.loadedAt.IsZero() {
			m.lastUpdated = msg.loadedAt
		}

	case refreshTickMsg:
		return m, tea.Batch(m.loadCurrentView(true), tick(m.refreshInterval))
	}

	return m, cmd
}

func (m Model) loadCurrentView(force bool) tea.Cmd {
	return m.loadDashboard(force)
}

func (m Model) loadDashboard(force bool) tea.Cmd {
	date := m.selectedDate
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		matches, matchesErr := m.matchService.Matches(ctx, date, force)
		standings, standingsErr := m.standingService.Standings(ctx, force)

		return dataLoadedMsg{
			matches:     matches,
			matchesOK:   matchesErr == nil,
			standings:   standings,
			standingsOK: standingsErr == nil,
			err:         joinErrors(matchesErr, standingsErr),
			loadedAt:    time.Now(),
		}
	}
}

func (m Model) loadMatchDetails(matchID string, force bool) tea.Cmd {
	date := m.selectedDate
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		details, err := m.matchService.MatchDetails(ctx, date, matchID, force)
		return detailsLoadedMsg{
			details:  details,
			err:      err,
			loadedAt: time.Now(),
		}
	}
}

func tick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

func (m *Model) nextTopLevelView() {
	if m.currentView == ViewLiveMatches {
		m.currentView = ViewStandings
	} else {
		m.currentView = ViewLiveMatches
	}
	m.viewport.GotoTop()
}

func (m *Model) ensureSelectedVisible() {
	if m.currentView != ViewLiveMatches || m.viewport.Height <= 0 {
		return
	}

	const liveHeaderRows = 2
	selectedLine := liveHeaderRows + m.selected
	if selectedLine < m.viewport.YOffset {
		m.viewport.SetYOffset(selectedLine)
		return
	}

	bottom := m.viewport.YOffset + m.viewport.Height - 1
	if selectedLine > bottom {
		m.viewport.SetYOffset(selectedLine - m.viewport.Height + 1)
	}
}

func (m *Model) shiftSelectedDate(days int) {
	m.selectedDate = startOfDay(m.selectedDate.AddDate(0, 0, days))
	m.selected = 0
	m.selectedMatchID = ""
	m.details = types.MatchDetails{}
	m.viewport.GotoTop()
}

func (m *Model) moveSelection(delta int) {
	matches := m.matchesForSelectedDate()
	if len(matches) == 0 {
		m.selected = 0
		return
	}

	m.selected = max(0, min(len(matches)-1, m.selected+delta))
}

func (m *Model) clampSelection() {
	matches := m.matchesForSelectedDate()
	if len(matches) == 0 {
		m.selected = 0
		return
	}

	m.selected = max(0, min(len(matches)-1, m.selected))
}

func (m Model) selectedMatch() (types.Match, bool) {
	matches := m.matchesForSelectedDate()
	if len(matches) == 0 || m.selected < 0 || m.selected >= len(matches) {
		return types.Match{}, false
	}

	return matches[m.selected], true
}

func startOfDay(t time.Time) time.Time {
	local := t.Local()
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

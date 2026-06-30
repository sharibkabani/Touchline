package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"touchline/internal/types"
)

const requestTimeout = 10 * time.Second

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncScrollViewport()
		m.ensureSelectedVisible()

	case tea.MouseMsg:
		// Trackpad/wheel scrolling for the standings and bracket views. The
		// dashboard paginates its own panes, so the viewport is bypassed there.
		if m.currentView != ViewLiveMatches {
			m.viewport, cmd = m.viewport.Update(msg)
		}

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
			m.syncScrollViewport()
			m.ensureSelectedVisible()
		case "l", "L":
			// Toggle the detail pane between stats/timeline and the on-pitch
			// formation. Only meaningful on the live-matches dashboard; the
			// renderer falls back to stats when no lineup is available.
			if m.currentView == ViewLiveMatches {
				m.showLineups = !m.showLineups
			} else {
				m.viewport, cmd = m.viewport.Update(msg)
			}
		case "left":
			if m.currentView == ViewLiveMatches {
				m.shiftSelectedDate(-1)
				m.loading = true
				m.err = nil
				return m, m.loadDashboard(false)
			}
			m.viewport, cmd = m.viewport.Update(msg)
		case "right":
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
		if msg.bracketOK {
			m.bracket = msg.bracket
		}
		m.err = msg.err
		if !msg.loadedAt.IsZero() {
			m.lastUpdated = msg.loadedAt
		}
		m.syncScrollViewport()
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
	// The bracket is a separate (whole-tournament) request, so only fetch it when
	// it is actually needed: on first load, or while the bracket tab is open. A
	// bracket fetch error is surfaced only when that tab is showing, so it never
	// disrupts the matches or standings views.
	fetchBracket := m.currentView == ViewBracket || len(m.bracket) == 0
	bracketErrRelevant := m.currentView == ViewBracket
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		matches, matchesErr := m.matchService.Matches(ctx, date, force)
		standings, standingsErr := m.standingService.Standings(ctx, force)

		var bracket []types.BracketRound
		var bracketErr error
		bracketOK := false
		if fetchBracket {
			bracket, bracketErr = m.bracketService.Bracket(ctx, force)
			bracketOK = bracketErr == nil
		}

		errs := []error{matchesErr, standingsErr}
		if bracketErrRelevant {
			errs = append(errs, bracketErr)
		}

		return dataLoadedMsg{
			matches:     matches,
			matchesOK:   matchesErr == nil,
			standings:   standings,
			standingsOK: standingsErr == nil,
			bracket:     bracket,
			bracketOK:   bracketOK,
			err:         joinErrors(errs...),
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
	switch m.currentView {
	case ViewLiveMatches:
		m.currentView = ViewStandings
	case ViewStandings:
		m.currentView = ViewBracket
	default:
		m.currentView = ViewLiveMatches
	}
	m.viewport.GotoTop()
}

// syncScrollViewport loads the current scroll view's content (standings or
// bracket) and dimensions into the viewport so its key/wheel handling can page
// correctly. Without this the viewport receives key events but holds no content,
// so its offset stays pinned at zero and nothing scrolls. It is a no-op on the
// dashboard, which lays out its own panes.
func (m *Model) syncScrollViewport() {
	if m.currentView == ViewLiveMatches || m.width == 0 {
		return
	}
	body := m.renderBody()
	m.viewport.Height = m.availableBodyRows()
	m.viewport.Width = min(max(20, m.width-2), max(20, lipgloss.Width(body)))
	m.viewport.SetContent(body)
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

// displayLocation is the timezone used for all day grouping ("Today"/"Yesterday")
// and kickoff formatting. It defaults to US Eastern so a UTC host (such as the
// SSH server on Fly.io) still rolls over the match day at the right moment for
// the audience instead of at midnight UTC.
var displayLocation = mustLoadLocation("America/New_York")

func mustLoadLocation(name string) *time.Location {
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	return time.Local
}

// SetTimezone overrides the display timezone by IANA name (e.g.
// "America/New_York"). Empty or unknown names leave the current location in
// place.
func SetTimezone(name string) {
	if name == "" {
		return
	}
	if loc, err := time.LoadLocation(name); err == nil {
		displayLocation = loc
	}
}

func startOfDay(t time.Time) time.Time {
	local := t.In(displayLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

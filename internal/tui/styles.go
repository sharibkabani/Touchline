package tui

import (
	"github.com/charmbracelet/lipgloss"

	"touchline/internal/types"
)

var (
	green  = lipgloss.Color("42")
	white  = lipgloss.Color("255")
	yellow = lipgloss.Color("220")
	red    = lipgloss.Color("203")
	muted  = white
	dim    = lipgloss.Color("243")
	panel  = lipgloss.Color("236")

	appStyle = lipgloss.NewStyle().
			Foreground(white).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(green).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(green).
			MarginBottom(1)

	// selectedBarStyle renders the vertical cursor bar that sits to the left of
	// the highlighted match. It is drawn once per visual line of the row so the
	// bar reads as a single continuous gutter across the whole entry.
	selectedBarStyle = lipgloss.NewStyle().
				Foreground(green).
				Bold(true)

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(green).
				Bold(true)

	normalRowStyle = lipgloss.NewStyle().
			Foreground(white)

	mutedStyle = lipgloss.NewStyle().
			Foreground(muted)

	errorStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(green)

	helpStyle = lipgloss.NewStyle().
			Foreground(muted)

	liveStatusStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true).
			Padding(0, 1)

	halfTimeStatusStyle = lipgloss.NewStyle().
				Foreground(green).
				Bold(true).
				Padding(0, 1)

	fullTimeStatusStyle = lipgloss.NewStyle().
				Foreground(muted).
				Bold(true).
				Padding(0, 1)

	defaultStatusStyle = lipgloss.NewStyle().
				Foreground(white).
				Padding(0, 1)

	paneStyle = lipgloss.NewStyle().
			Foreground(white).
			Border(lipgloss.NormalBorder()).
			BorderForeground(green).
			Padding(0, 1)

	// bigScoreStyle colours the oversized ASCII scoreline that anchors the
	// match-details pane.
	bigScoreStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	scoreTeamStyle = lipgloss.NewStyle().
			Foreground(white).
			Bold(true)

	// Timeline markers: goals carry the green accent, cards use their real
	// colours, and the connecting line is dimmed so the nodes stand out.
	goalDotStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	yellowCardStyle = lipgloss.NewStyle().
			Foreground(yellow).
			Bold(true)

	redCardStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	timelineConnectorStyle = lipgloss.NewStyle().
				Foreground(dim)

	bannerStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(white)

	// groupCardStyle frames each group's standings table so the twelve groups
	// read as distinct cards instead of one dense block of numbers.
	groupCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(green).
			Padding(0, 1).
			MarginRight(1)

	groupTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(green)

	groupHeaderStyle = lipgloss.NewStyle().
				Foreground(dim)
)

func statusBadge(status types.MatchStatus) string {
	switch status {
	case types.StatusLive, types.StatusExtraTime:
		return liveStatusStyle.Render(string(status))
	case types.StatusHalfTime:
		return halfTimeStatusStyle.Render(string(status))
	case types.StatusFullTime:
		return fullTimeStatusStyle.Render(string(status))
	default:
		return defaultStatusStyle.Render(string(status))
	}
}

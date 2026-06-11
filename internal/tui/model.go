package tui

import (
	"errors"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"touchline/internal/services"
	"touchline/internal/types"
)

type View int

const (
	ViewLiveMatches View = iota
	ViewStandings
)

type Model struct {
	matchService    *services.MatchService
	standingService *services.StandingService
	refreshInterval time.Duration

	currentView  View
	selected     int
	selectedDate time.Time

	matches   []types.Match
	details   types.MatchDetails
	standings []types.GroupStanding

	selectedMatchID string
	loading         bool
	err             error
	lastUpdated     time.Time

	viewport viewport.Model
	width    int
	height   int
}

type dataLoadedMsg struct {
	matches     []types.Match
	matchesOK   bool
	standings   []types.GroupStanding
	standingsOK bool
	err         error
	loadedAt    time.Time
}

type detailsLoadedMsg struct {
	details  types.MatchDetails
	err      error
	loadedAt time.Time
}

type refreshTickMsg time.Time

func NewModel(
	matchService *services.MatchService,
	standingService *services.StandingService,
	refreshInterval time.Duration,
) Model {
	return Model{
		matchService:    matchService,
		standingService: standingService,
		refreshInterval: refreshInterval,
		currentView:     ViewLiveMatches,
		selectedDate:    startOfDay(time.Now()),
		loading:         true,
		viewport:        viewport.New(0, 0),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadDashboard(false), tick(m.refreshInterval))
}

func joinErrors(errs ...error) error {
	var joined error
	for _, err := range errs {
		if err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

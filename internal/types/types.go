package types

import "time"

// CompetitionCode keeps the application ready for additional tournaments
// without threading provider-specific identifiers through the TUI.
type CompetitionCode string

const (
	CompetitionWorldCup CompetitionCode = "world-cup"
)

type MatchStatus string

const (
	StatusLive      MatchStatus = "LIVE"
	StatusHalfTime  MatchStatus = "HT"
	StatusFullTime  MatchStatus = "FT"
	StatusExtraTime MatchStatus = "EXTRA TIME"
	StatusScheduled MatchStatus = "SCHEDULED"
)

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Score struct {
	Home int `json:"home"`
	Away int `json:"away"`
}

type Match struct {
	ID          string          `json:"id"`
	Competition CompetitionCode `json:"competition"`
	Group       string          `json:"group,omitempty"`
	Stage       string          `json:"stage"`
	HomeTeam    Team            `json:"homeTeam"`
	AwayTeam    Team            `json:"awayTeam"`
	Score       Score           `json:"score"`
	Minute      int             `json:"minute"`
	Status      MatchStatus     `json:"status"`
	Kickoff     time.Time       `json:"kickoff,omitempty"`
}

type MatchStatistics struct {
	PossessionHome    int `json:"possessionHome"`
	PossessionAway    int `json:"possessionAway"`
	ShotsHome         int `json:"shotsHome"`
	ShotsAway         int `json:"shotsAway"`
	ShotsOnTargetHome int `json:"shotsOnTargetHome"`
	ShotsOnTargetAway int `json:"shotsOnTargetAway"`
	CornersHome       int `json:"cornersHome"`
	CornersAway       int `json:"cornersAway"`
	OffsidesHome      int `json:"offsidesHome"`
	OffsidesAway      int `json:"offsidesAway"`
	SavesHome         int `json:"savesHome"`
	SavesAway         int `json:"savesAway"`
	FoulsHome         int `json:"foulsHome"`
	FoulsAway         int `json:"foulsAway"`
	YellowCardsHome   int `json:"yellowCardsHome"`
	YellowCardsAway   int `json:"yellowCardsAway"`
	RedCardsHome      int `json:"redCardsHome"`
	RedCardsAway      int `json:"redCardsAway"`
}

// MatchEventType keeps the timeline provider-neutral so any data source can map
// its own event vocabulary onto a small, renderable set.
type MatchEventType string

const (
	EventGoal    MatchEventType = "GOAL"
	EventOwnGoal MatchEventType = "OWN GOAL"
	EventPenalty MatchEventType = "PENALTY"
	EventYellow  MatchEventType = "YELLOW"
	EventRed     MatchEventType = "RED"
)

// MatchEvent is a single timeline entry such as a goal or card.
type MatchEvent struct {
	Minute string         `json:"minute"`
	Type   MatchEventType `json:"type"`
	Text   string         `json:"text"`
	Player string         `json:"player"`
	Team   string         `json:"team"`
	Home   bool           `json:"home"`
}

// LineupPlayer is a single entry in a team sheet. Number and Position are best
// effort: providers do not always supply them, especially for substitutes.
type LineupPlayer struct {
	Name     string `json:"name"`
	Number   int    `json:"number,omitempty"`
	Position string `json:"position,omitempty"`
	Starter  bool   `json:"starter"`
}

// TeamLineup is one side's expected or confirmed team sheet. It is empty until
// the lineup is announced (typically about an hour before kickoff).
type TeamLineup struct {
	Formation string         `json:"formation,omitempty"`
	Players   []LineupPlayer `json:"players,omitempty"`
}

type MatchDetails struct {
	Match      Match           `json:"match"`
	Statistics MatchStatistics `json:"statistics"`
	Events     []MatchEvent    `json:"events,omitempty"`
	Venue      string          `json:"venue,omitempty"`
	League     string          `json:"league,omitempty"`
	Attendance int             `json:"attendance,omitempty"`
	Referee    string          `json:"referee,omitempty"`
	HomeLineup TeamLineup      `json:"homeLineup,omitempty"`
	AwayLineup TeamLineup      `json:"awayLineup,omitempty"`
}

// Scoreboard bundles a day's matches with their details. Providers that expose
// everything in one upstream response (like ESPN) can populate both together,
// keeping a day to a single network request.
type Scoreboard struct {
	Matches []Match                 `json:"matches"`
	Details map[string]MatchDetails `json:"details"`
}

type StandingRow struct {
	Position       int  `json:"position"`
	Team           Team `json:"team"`
	Played         int  `json:"played"`
	Won            int  `json:"won"`
	Drawn          int  `json:"drawn"`
	Lost           int  `json:"lost"`
	GoalDifference int  `json:"goalDifference"`
	Points         int  `json:"points"`
}

type GroupStanding struct {
	Group string        `json:"group"`
	Rows  []StandingRow `json:"rows"`
}

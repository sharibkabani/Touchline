package api

// These types mirror the subset of ESPN's public soccer JSON that Touchline
// consumes. They are deliberately unexported: nothing outside this package
// should depend on ESPN's wire format.

type espnScoreboard struct {
	Leagues []espnLeague `json:"leagues"`
	Events  []espnEvent  `json:"events"`
}

type espnLeague struct {
	Name string `json:"name"`
}

type espnEvent struct {
	ID           string            `json:"id"`
	Date         string            `json:"date"`
	Name         string            `json:"name"`
	Competitions []espnCompetition `json:"competitions"`
}

type espnCompetition struct {
	Venue       espnVenue        `json:"venue"`
	Status      espnStatus       `json:"status"`
	Notes       []espnNote       `json:"notes"`
	Competitors []espnCompetitor `json:"competitors"`
	Details     []espnDetail     `json:"details"`
}

type espnVenue struct {
	FullName string `json:"fullName"`
	Address  struct {
		City    string `json:"city"`
		Country string `json:"country"`
	} `json:"address"`
}

type espnNote struct {
	Headline string `json:"headline"`
}

type espnStatus struct {
	DisplayClock string `json:"displayClock"`
	Type         struct {
		State       string `json:"state"`
		Detail      string `json:"detail"`
		ShortDetail string `json:"shortDetail"`
		Completed   bool   `json:"completed"`
	} `json:"type"`
}

type espnCompetitor struct {
	ID         string         `json:"id"`
	HomeAway   string         `json:"homeAway"`
	Score      string         `json:"score"`
	Team       espnTeam       `json:"team"`
	Statistics []espnStatItem `json:"statistics"`
}

type espnTeam struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	ShortDisplayName string `json:"shortDisplayName"`
	Abbreviation     string `json:"abbreviation"`
}

type espnStatItem struct {
	Name         string `json:"name"`
	DisplayValue string `json:"displayValue"`
}

type espnDetail struct {
	Type struct {
		Text string `json:"text"`
	} `json:"type"`
	Clock struct {
		DisplayValue string `json:"displayValue"`
	} `json:"clock"`
	Team struct {
		ID string `json:"id"`
	} `json:"team"`
	ScoringPlay      bool `json:"scoringPlay"`
	RedCard          bool `json:"redCard"`
	YellowCard       bool `json:"yellowCard"`
	PenaltyKick      bool `json:"penaltyKick"`
	OwnGoal          bool `json:"ownGoal"`
	AthletesInvolved []struct {
		DisplayName string `json:"displayName"`
		ShortName   string `json:"shortName"`
	} `json:"athletesInvolved"`
}

// espnSummary is the event summary response. It carries the team sheets under
// rosters[] once they are published, and the goal/card timeline under
// commentary[] (the fifa.world scoreboard endpoint omits play details).
type espnSummary struct {
	Rosters    []espnRoster     `json:"rosters"`
	Commentary []espnCommentary `json:"commentary"`
	GameInfo   espnGameInfo     `json:"gameInfo"`
	Boxscore   espnBoxscore     `json:"boxscore"`
}

// espnGameInfo carries the contextual facts ESPN attaches to a fixture:
// attendance, the match officials, and a richer venue (with city/country).
type espnGameInfo struct {
	Attendance int       `json:"attendance"`
	Venue      espnVenue `json:"venue"`
	Officials  []struct {
		DisplayName string `json:"displayName"`
		Position    struct {
			DisplayName string `json:"displayName"`
		} `json:"position"`
	} `json:"officials"`
}

// espnBoxscore exposes the full per-team statistic set (the scoreboard endpoint
// only returns a subset), keyed by home/away.
type espnBoxscore struct {
	Teams []struct {
		HomeAway   string         `json:"homeAway"`
		Statistics []espnStatItem `json:"statistics"`
	} `json:"teams"`
}

type espnCommentary struct {
	Time struct {
		DisplayValue string `json:"displayValue"`
	} `json:"time"`
	Text string   `json:"text"`
	Play espnPlay `json:"play"`
}

type espnPlay struct {
	Type struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"type"`
	Text        string `json:"text"`
	ScoringPlay bool   `json:"scoringPlay"`
	Team        struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"team"`
	Participants []struct {
		Athlete struct {
			DisplayName string `json:"displayName"`
		} `json:"athlete"`
	} `json:"participants"`
}

type espnRoster struct {
	HomeAway  string            `json:"homeAway"`
	Formation string            `json:"formation"`
	Team      espnTeam          `json:"team"`
	Roster    []espnRosterEntry `json:"roster"`
}

type espnRosterEntry struct {
	Starter bool   `json:"starter"`
	Jersey  string `json:"jersey"`
	Athlete struct {
		DisplayName string `json:"displayName"`
		ShortName   string `json:"shortName"`
	} `json:"athlete"`
	Position struct {
		Abbreviation string `json:"abbreviation"`
	} `json:"position"`
}

type espnStandings struct {
	Children []struct {
		Name      string `json:"name"`
		Standings struct {
			Entries []struct {
				Team struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
				} `json:"team"`
				Stats []struct {
					Name         string `json:"name"`
					Type         string `json:"type"`
					DisplayValue string `json:"displayValue"`
				} `json:"stats"`
			} `json:"entries"`
		} `json:"standings"`
	} `json:"children"`
}

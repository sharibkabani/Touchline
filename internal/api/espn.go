package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"touchline/internal/types"
)

const (
	defaultESPNScoreboardBase = "https://site.api.espn.com/apis/site/v2/sports/soccer"
	defaultESPNStandingsBase  = "https://site.api.espn.com/apis/v2/sports/soccer"
)

// ESPNProvider talks to ESPN's public soccer endpoints. The scoreboard call
// already returns per-match statistics and a goal/card timeline, so a single
// request per day yields both the match list and full match details.
type ESPNProvider struct {
	scoreboardBase string
	standingsBase  string
	httpClient     *http.Client
}

func NewESPNProvider(baseURL string) *ESPNProvider {
	scoreboardBase := baseURL
	if scoreboardBase == "" {
		scoreboardBase = defaultESPNScoreboardBase
	}

	return &ESPNProvider{
		scoreboardBase: scoreboardBase,
		standingsBase:  defaultESPNStandingsBase,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

// espnSlug maps our provider-neutral competition codes onto ESPN league slugs.
func espnSlug(competition types.CompetitionCode) string {
	switch competition {
	case types.CompetitionWorldCup:
		return "fifa.world"
	default:
		return string(competition)
	}
}

func (p *ESPNProvider) GetScoreboard(ctx context.Context, competition types.CompetitionCode, date time.Time) (types.Scoreboard, error) {
	url := fmt.Sprintf("%s/%s/scoreboard?dates=%s", p.scoreboardBase, espnSlug(competition), date.Format("20060102"))

	var response espnScoreboard
	if err := p.getJSON(ctx, url, &response); err != nil {
		return types.Scoreboard{}, err
	}

	league := ""
	if len(response.Leagues) > 0 {
		league = response.Leagues[0].Name
	}

	scoreboard := types.Scoreboard{
		Matches: make([]types.Match, 0, len(response.Events)),
		Details: make(map[string]types.MatchDetails, len(response.Events)),
	}

	for _, event := range response.Events {
		match, details, ok := convertEvent(event, league, competition)
		if !ok {
			continue
		}
		scoreboard.Matches = append(scoreboard.Matches, match)
		scoreboard.Details[match.ID] = details
	}

	p.enrichDetails(ctx, competition, &scoreboard)

	return scoreboard, nil
}

// enrichDetails fetches each match's summary (one call per match, concurrently)
// to fill in data the fifa.world scoreboard omits: expected team sheets and the
// goal/card timeline. Failures are non-fatal — a match simply keeps whatever the
// scoreboard already provided.
func (p *ESPNProvider) enrichDetails(ctx context.Context, competition types.CompetitionCode, scoreboard *types.Scoreboard) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range scoreboard.Matches {
		match := scoreboard.Matches[i]

		wg.Add(1)
		go func(m types.Match) {
			defer wg.Done()

			summary, err := p.getSummary(ctx, competition, m.ID)
			if err != nil {
				return
			}

			home, away := summaryLineups(summary)
			events := buildCommentaryEvents(summary.Commentary, m.HomeTeam.Name, m.AwayTeam.Name)

			mu.Lock()
			details := scoreboard.Details[m.ID]
			details.HomeLineup = home
			details.AwayLineup = away
			if len(events) > 0 {
				details.Events = events
				applyCardCounts(&details.Statistics, events)
			}
			scoreboard.Details[m.ID] = details
			mu.Unlock()
		}(match)
	}

	wg.Wait()
}

func (p *ESPNProvider) getSummary(ctx context.Context, competition types.CompetitionCode, eventID string) (espnSummary, error) {
	url := fmt.Sprintf("%s/%s/summary?event=%s", p.scoreboardBase, espnSlug(competition), eventID)

	var response espnSummary
	if err := p.getJSON(ctx, url, &response); err != nil {
		return espnSummary{}, err
	}

	return response, nil
}

func summaryLineups(summary espnSummary) (home, away types.TeamLineup) {
	for _, roster := range summary.Rosters {
		switch roster.HomeAway {
		case "home":
			home = convertLineup(roster)
		case "away":
			away = convertLineup(roster)
		}
	}
	return home, away
}

// buildCommentaryEvents distills the prose commentary feed into a goal/card
// timeline. ESPN's structured play data lags the live text, so each entry is
// classified from play.type when present and from the text otherwise.
func buildCommentaryEvents(items []espnCommentary, homeName, awayName string) []types.MatchEvent {
	events := make([]types.MatchEvent, 0, len(items))
	seen := make(map[string]bool, len(items))

	for _, item := range items {
		eventType, ok := classifyCommentary(item)
		if !ok {
			continue
		}

		player := ""
		if len(item.Play.Participants) > 0 {
			player = item.Play.Participants[0].Athlete.DisplayName
		}
		team := item.Play.Team.DisplayName
		if player == "" || team == "" {
			parsedPlayer, parsedTeam := parseEventText(item.Text)
			if player == "" {
				player = parsedPlayer
			}
			if team == "" {
				team = parsedTeam
			}
		}

		minute := item.Time.DisplayValue
		key := string(eventType) + "|" + minute + "|" + player
		if seen[key] {
			continue
		}
		seen[key] = true

		events = append(events, types.MatchEvent{
			Minute: minute,
			Type:   eventType,
			Text:   item.Text,
			Player: player,
			Team:   team,
			Home:   isHomeTeam(team, homeName, awayName),
		})
	}

	return events
}

// classifyCommentary maps a commentary entry onto a timeline event type, or
// reports false for entries that are not goals or cards.
func classifyCommentary(item espnCommentary) (types.MatchEventType, bool) {
	typeText := strings.ToLower(item.Play.Type.Text)
	text := strings.ToLower(item.Text)

	switch {
	case strings.Contains(typeText, "red card"):
		return types.EventRed, true
	case strings.Contains(typeText, "yellow card"):
		return types.EventYellow, true
	case strings.Contains(typeText, "goal"):
		switch {
		case strings.Contains(typeText, "own goal") || strings.Contains(text, "own goal"):
			return types.EventOwnGoal, true
		case strings.Contains(typeText, "penalty") || strings.Contains(text, "penalty"):
			return types.EventPenalty, true
		default:
			return types.EventGoal, true
		}
	}

	// Fallback for live entries that arrive before structured play data.
	switch {
	case strings.HasPrefix(text, "goal!"):
		switch {
		case strings.Contains(text, "own goal"):
			return types.EventOwnGoal, true
		case strings.Contains(text, "penalty"):
			return types.EventPenalty, true
		default:
			return types.EventGoal, true
		}
	case strings.Contains(text, "second yellow") || strings.Contains(text, "red card"):
		return types.EventRed, true
	case strings.Contains(text, "yellow card"):
		return types.EventYellow, true
	}

	return "", false
}

// parseEventText pulls the player and team out of commentary prose such as
// "Goal! Mexico 1, South Africa 0. Julián Quiñones (Mexico) ..." or
// "Sander Berge (Fulham) is shown the yellow card ...".
func parseEventText(text string) (player, team string) {
	open := strings.Index(text, "(")
	closeIdx := strings.Index(text, ")")
	if open < 0 || closeIdx <= open {
		return "", ""
	}

	team = strings.TrimSpace(text[open+1 : closeIdx])

	prefix := strings.TrimSpace(text[:open])
	if dot := strings.LastIndex(prefix, ". "); dot >= 0 {
		prefix = strings.TrimSpace(prefix[dot+2:])
	}
	player = prefix

	return player, team
}

func isHomeTeam(team, homeName, awayName string) bool {
	team = strings.TrimSpace(team)
	switch {
	case team == "":
		return false
	case strings.EqualFold(team, homeName):
		return true
	case strings.EqualFold(team, awayName):
		return false
	case homeName != "" && strings.Contains(strings.ToLower(homeName), strings.ToLower(team)):
		return true
	default:
		return false
	}
}

func applyCardCounts(stats *types.MatchStatistics, events []types.MatchEvent) {
	stats.YellowCardsHome, stats.YellowCardsAway = 0, 0
	stats.RedCardsHome, stats.RedCardsAway = 0, 0
	for _, event := range events {
		switch event.Type {
		case types.EventYellow:
			if event.Home {
				stats.YellowCardsHome++
			} else {
				stats.YellowCardsAway++
			}
		case types.EventRed:
			if event.Home {
				stats.RedCardsHome++
			} else {
				stats.RedCardsAway++
			}
		}
	}
}

func convertLineup(roster espnRoster) types.TeamLineup {
	players := make([]types.LineupPlayer, 0, len(roster.Roster))
	for _, entry := range roster.Roster {
		name := entry.Athlete.DisplayName
		if name == "" {
			name = entry.Athlete.ShortName
		}
		if name == "" {
			continue
		}

		players = append(players, types.LineupPlayer{
			Name:     name,
			Number:   atoi(entry.Jersey),
			Position: entry.Position.Abbreviation,
			Starter:  entry.Starter,
		})
	}

	return types.TeamLineup{Formation: roster.Formation, Players: players}
}

func (p *ESPNProvider) GetStandings(ctx context.Context, competition types.CompetitionCode) ([]types.GroupStanding, error) {
	url := fmt.Sprintf("%s/%s/standings", p.standingsBase, espnSlug(competition))

	var response espnStandings
	if err := p.getJSON(ctx, url, &response); err != nil {
		return nil, err
	}

	groups := make([]types.GroupStanding, 0, len(response.Children))
	for _, child := range response.Children {
		rows := make([]types.StandingRow, 0, len(child.Standings.Entries))
		for _, entry := range child.Standings.Entries {
			row := types.StandingRow{
				Team: types.Team{ID: entry.Team.ID, Name: entry.Team.DisplayName},
			}
			for _, stat := range entry.Stats {
				switch stat.Type {
				case "rank":
					row.Position = atoi(stat.DisplayValue)
				case "gamesplayed":
					row.Played = atoi(stat.DisplayValue)
				case "wins":
					row.Won = atoi(stat.DisplayValue)
				case "ties":
					row.Drawn = atoi(stat.DisplayValue)
				case "losses":
					row.Lost = atoi(stat.DisplayValue)
				case "pointdifferential":
					row.GoalDifference = atoi(stat.DisplayValue)
				case "points":
					row.Points = atoi(stat.DisplayValue)
				}
			}
			rows = append(rows, row)
		}

		if len(rows) > 0 {
			groups = append(groups, types.GroupStanding{Group: child.Name, Rows: rows})
		}
	}

	return groups, nil
}

func (p *ESPNProvider) getJSON(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build espn request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("espn request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("espn responded with status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode espn response: %w", err)
	}

	return nil
}

// convertEvent maps one ESPN event into our domain match plus full details.
func convertEvent(event espnEvent, league string, competition types.CompetitionCode) (types.Match, types.MatchDetails, bool) {
	if len(event.Competitions) == 0 {
		return types.Match{}, types.MatchDetails{}, false
	}

	comp := event.Competitions[0]
	var home, away *espnCompetitor
	for i := range comp.Competitors {
		switch comp.Competitors[i].HomeAway {
		case "home":
			home = &comp.Competitors[i]
		case "away":
			away = &comp.Competitors[i]
		}
	}
	if home == nil || away == nil {
		return types.Match{}, types.MatchDetails{}, false
	}

	status, minute := mapStatus(comp.Status)

	stage := league
	if len(comp.Notes) > 0 && comp.Notes[0].Headline != "" {
		stage = comp.Notes[0].Headline
	}

	match := types.Match{
		ID:          event.ID,
		Competition: competition,
		Stage:       stage,
		HomeTeam:    types.Team{ID: home.Team.ID, Name: teamName(home.Team)},
		AwayTeam:    types.Team{ID: away.Team.ID, Name: teamName(away.Team)},
		Score:       types.Score{Home: atoi(home.Score), Away: atoi(away.Score)},
		Minute:      minute,
		Status:      status,
		Kickoff:     parseESPNTime(event.Date),
	}

	stats := buildStatistics(home, away)
	events := buildEvents(comp.Details, home.Team.ID, teamName(home.Team), teamName(away.Team))
	for _, ev := range events {
		switch ev.Type {
		case types.EventYellow:
			if ev.Home {
				stats.YellowCardsHome++
			} else {
				stats.YellowCardsAway++
			}
		case types.EventRed:
			if ev.Home {
				stats.RedCardsHome++
			} else {
				stats.RedCardsAway++
			}
		}
	}

	venue := comp.Venue.FullName
	if comp.Venue.Address.City != "" {
		venue = strings.TrimSpace(strings.TrimPrefix(venue+", "+comp.Venue.Address.City, ", "))
	}

	details := types.MatchDetails{
		Match:      match,
		Statistics: stats,
		Events:     events,
		Venue:      venue,
		League:     league,
	}

	return match, details, true
}

func teamName(team espnTeam) string {
	if team.DisplayName != "" {
		return team.DisplayName
	}
	if team.ShortDisplayName != "" {
		return team.ShortDisplayName
	}
	return team.Abbreviation
}

func mapStatus(status espnStatus) (types.MatchStatus, int) {
	detail := strings.ToUpper(status.Type.Detail + " " + status.Type.ShortDetail)

	switch status.Type.State {
	case "post":
		return types.StatusFullTime, 0
	case "pre":
		return types.StatusScheduled, 0
	case "in":
		switch {
		case strings.Contains(detail, "HALF") || strings.Contains(detail, "HT"):
			return types.StatusHalfTime, parseMinute(status.DisplayClock)
		case strings.Contains(detail, "ET") || strings.Contains(detail, "EXTRA"):
			return types.StatusExtraTime, parseMinute(status.DisplayClock)
		default:
			return types.StatusLive, parseMinute(status.DisplayClock)
		}
	default:
		return types.StatusScheduled, 0
	}
}

func buildStatistics(home, away *espnCompetitor) types.MatchStatistics {
	var stats types.MatchStatistics
	for _, stat := range home.Statistics {
		applyStat(&stats, stat.Name, stat.DisplayValue, true)
	}
	for _, stat := range away.Statistics {
		applyStat(&stats, stat.Name, stat.DisplayValue, false)
	}
	return stats
}

func applyStat(stats *types.MatchStatistics, name, value string, home bool) {
	switch name {
	case "possessionPct":
		set(&stats.PossessionHome, &stats.PossessionAway, home, atoiRound(value))
	case "totalShots":
		set(&stats.ShotsHome, &stats.ShotsAway, home, atoi(value))
	case "shotsOnTarget":
		set(&stats.ShotsOnTargetHome, &stats.ShotsOnTargetAway, home, atoi(value))
	case "wonCorners":
		set(&stats.CornersHome, &stats.CornersAway, home, atoi(value))
	case "foulsCommitted":
		set(&stats.FoulsHome, &stats.FoulsAway, home, atoi(value))
	}
}

func set(homeField, awayField *int, home bool, value int) {
	if home {
		*homeField = value
	} else {
		*awayField = value
	}
}

func buildEvents(details []espnDetail, homeID, homeName, awayName string) []types.MatchEvent {
	events := make([]types.MatchEvent, 0, len(details))
	for _, detail := range details {
		var eventType types.MatchEventType
		switch {
		case detail.OwnGoal:
			eventType = types.EventOwnGoal
		case detail.ScoringPlay && detail.PenaltyKick:
			eventType = types.EventPenalty
		case detail.ScoringPlay:
			eventType = types.EventGoal
		case detail.RedCard:
			eventType = types.EventRed
		case detail.YellowCard:
			eventType = types.EventYellow
		default:
			continue
		}

		player := ""
		if len(detail.AthletesInvolved) > 0 {
			player = detail.AthletesInvolved[0].DisplayName
			if player == "" {
				player = detail.AthletesInvolved[0].ShortName
			}
		}

		home := detail.Team.ID == homeID
		team := awayName
		if home {
			team = homeName
		}

		events = append(events, types.MatchEvent{
			Minute: detail.Clock.DisplayValue,
			Type:   eventType,
			Text:   detail.Type.Text,
			Player: player,
			Team:   team,
			Home:   home,
		})
	}
	return events
}

func atoi(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return n
}

func atoiRound(value string) int {
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return int(f + 0.5)
}

// parseMinute extracts the leading number from clocks like "73'" or "90'+6'".
func parseMinute(clock string) int {
	digits := strings.Builder{}
	for _, r := range clock {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
			continue
		}
		break
	}
	return atoi(digits.String())
}

func parseESPNTime(value string) time.Time {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15:04Z",
		"2006-01-02T15:04:05Z",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

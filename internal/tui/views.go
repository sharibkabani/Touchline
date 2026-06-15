package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"touchline/internal/types"
)

// touchlineBanner is an ANSI Shadow rendering of the app name. It is the
// centerpiece header and is shown whenever the terminal is wide enough to hold
// it without wrapping; otherwise a compact fallback title is used.
const touchlineBanner = `████████╗ ██████╗ ██╗   ██╗ ██████╗██╗  ██╗██╗     ██╗███╗   ██╗███████╗
╚══██╔══╝██╔═══██╗██║   ██║██╔════╝██║  ██║██║     ██║████╗  ██║██╔════╝
   ██║   ██║   ██║██║   ██║██║     ███████║██║     ██║██╔██╗ ██║█████╗  
   ██║   ██║   ██║██║   ██║██║     ██╔══██║██║     ██║██║╚██╗██║██╔══╝  
   ██║   ╚██████╔╝╚██████╔╝╚██████╗██║  ██║███████╗██║██║ ╚████║███████╗
   ╚═╝    ╚═════╝  ╚═════╝  ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝╚═╝  ╚═══╝╚══════╝`

func (m Model) View() string {
	if m.width == 0 {
		return "Loading Touchline..."
	}

	banner := m.renderBanner()
	status := m.renderStatusLine()
	footer := m.renderFooter()

	// Reserve space for the banner, status line, footer, and the blank spacer
	// rows so the scrollable/pane body never pushes content off screen.
	chrome := lipgloss.Height(banner) + lipgloss.Height(status) + lipgloss.Height(footer) + 4
	availableRows := max(6, m.height-chrome)

	var body string
	if m.currentView == ViewLiveMatches {
		body = m.renderDashboardBody(availableRows)
	} else {
		body = m.renderScrollBody(availableRows)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		banner,
		status,
		"",
		body,
		"",
		footer,
	)

	// Place centers the entire composed view horizontally within the terminal.
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, content)
}

func (m Model) renderBanner() string {
	if m.width >= lipgloss.Width(touchlineBanner) {
		return lipgloss.JoinVertical(
			lipgloss.Center,
			bannerStyle.Render(touchlineBanner),
			subtitleStyle.Render("FIFA World Cup"),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Center,
		bannerStyle.Render("T O U C H L I N E"),
		subtitleStyle.Render("FIFA World Cup"),
	)
}

func (m Model) renderStatusLine() string {
	parts := []string{strings.ToUpper(m.currentViewLabel())}
	if m.loading {
		parts = append(parts, "refreshing")
	}
	if !m.lastUpdated.IsZero() {
		parts = append(parts, "updated "+m.lastUpdated.Format(time.Kitchen))
	}

	return mutedStyle.Render(strings.Join(parts, "  |  "))
}

func (m Model) renderScrollBody(availableRows int) string {
	body := m.renderBody()

	// When the content fits, center it vertically so the group cards sit in the
	// middle of the screen instead of being pinned to the top. Only fall back to
	// the scrolling viewport when the content is too tall to show at once.
	if lipgloss.Height(body) <= availableRows {
		return lipgloss.Place(lipgloss.Width(body), availableRows, lipgloss.Center, lipgloss.Center, body)
	}

	vp := m.viewport
	vp.Width = min(max(20, m.width-2), max(20, lipgloss.Width(body)))
	vp.Height = availableRows
	vp.SetContent(body)
	return vp.View()
}

func (m Model) renderFooter() string {
	help := "left/right day | up/down match | tab standings | r refresh | q quit"
	if m.currentView != ViewLiveMatches {
		help = "tab matches | up/down scroll | r refresh | q quit"
	} else if m.details.Match.Status != types.StatusScheduled &&
		(len(m.details.HomeLineup.Players) > 0 || len(m.details.AwayLineup.Players) > 0) {
		toggle := "l lineup"
		if m.showLineups {
			toggle = "l stats"
		}
		help = "left/right day | up/down match | " + toggle + " | tab standings | r refresh | q quit"
	}

	if m.err != nil {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			errorStyle.Render("Failed to refresh match data."),
			mutedStyle.Render("Press R to retry. "+m.err.Error()),
			helpStyle.Render(help),
		)
	}

	return helpStyle.Render(help)
}

// Pane chrome. lipgloss keeps Padding *inside* a style's Width but adds the
// Border *outside* it, so to expose exactly N columns of text we set the style
// width to N+panePadWidth and the box renders N+paneFrameWidth columns wide.
const (
	panePadWidth   = 2                // Padding(0, 1): one column each side
	paneFrameWidth = panePadWidth + 2 // padding + NormalBorder (one column each side)
)

// renderPane frames content in the bordered box, sizing it so exactly textWidth
// columns are available for content. This prevents lipgloss from auto-wrapping
// (and thus growing) the pane, and makes the box exactly textWidth+paneFrameWidth
// columns wide. vAlign positions content shorter than the box vertically (e.g.
// Top for the scrolling match list, Center for the detail card).
func (m Model) renderPane(content string, textWidth, height int, vAlign lipgloss.Position) string {
	return paneStyle.
		Width(textWidth + panePadWidth).
		Height(height).
		AlignVertical(vAlign).
		Render(clampHeight(content, height))
}

// renderDashboardBody lays out the live match list and detail panes. It returns
// only the panes (no banner/footer) so the caller can center the whole view.
func (m Model) renderDashboardBody(availableRows int) string {
	// Cap the overall width so the dashboard stays a centered, well-proportioned
	// card on very wide terminals instead of stretching edge to edge.
	contentWidth := max(32, min(m.width-2, 150))

	if contentWidth < 88 {
		textWidth := max(20, contentWidth-paneFrameWidth)
		// Stack the panes but keep both boxes the same height so they read as a
		// matching pair just like the side-by-side layout.
		paneHeight := max(5, (availableRows-2)/2)
		list := m.renderPane(m.renderMatchListPane(textWidth, paneHeight), textWidth, paneHeight, lipgloss.Top)
		details := m.renderPane(m.renderDetailContent(textWidth, paneHeight), textWidth, paneHeight, lipgloss.Center)
		return lipgloss.JoinVertical(lipgloss.Center, list, details)
	}

	leftOuterWidth := clamp(contentWidth/3, 30, 50)
	rightOuterWidth := max(36, contentWidth-leftOuterWidth-1)
	leftTextWidth := max(20, leftOuterWidth-paneFrameWidth)
	rightTextWidth := max(20, rightOuterWidth-paneFrameWidth)
	// Both panes share the exact same height so the rectangles are identical.
	paneHeight := availableRows
	left := m.renderPane(m.renderMatchListPane(leftTextWidth, paneHeight), leftTextWidth, paneHeight, lipgloss.Top)
	right := m.renderPane(m.renderDetailContent(rightTextWidth, paneHeight), rightTextWidth, paneHeight, lipgloss.Center)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

// renderMatchRow draws a single match entry with a consistent two-column
// gutter. The gutter is the same width whether or not the row is selected, so
// switching selection never reflows the text. When selected, the gutter becomes
// a green bar repeated on every visual line, producing one continuous cursor
// that encompasses the whole entry.
func renderMatchRow(content string, selected bool) string {
	textStyle := normalRowStyle
	gutter := "  "
	if selected {
		textStyle = selectedRowStyle
		gutter = selectedBarStyle.Render("\u2590") + " "
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = gutter + textStyle.Render(line)
	}
	return strings.Join(lines, "\n")
}

// clampHeight guarantees rendered pane content never exceeds the box height,
// which keeps the bordered rectangles from growing past their intended size.
func clampHeight(content string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderBody() string {
	if m.currentView == ViewStandings {
		return m.renderStandings()
	}
	return m.renderLiveMatches()
}

func (m Model) renderLiveMatches() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("LIVE MATCHES"))
	b.WriteString("\n\n")

	matches := m.matchesForSelectedDate()
	if len(matches) == 0 {
		b.WriteString(mutedStyle.Render("No World Cup matches for this day."))
		return b.String()
	}

	for i, match := range matches {
		score := fmt.Sprintf("%d - %d", match.Score.Home, match.Score.Away)
		if match.Status == types.StatusScheduled {
			score = "vs"
		}
		line := fmt.Sprintf(
			"%s %s %s  %s  %s",
			match.HomeTeam.Name,
			score,
			match.AwayTeam.Name,
			formatMinute(match),
			string(match.Status),
		)

		b.WriteString(renderMatchRow(line, i == m.selected))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderMatchListPane(width, height int) string {
	matches := m.matchesForSelectedDate()

	var b strings.Builder
	b.WriteString(sectionTitle("Match List", width))
	b.WriteString("\n")
	b.WriteString(renderDateTabs(m.selectedDate))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("%d items", len(matches))))
	b.WriteString("\n\n")

	if len(matches) == 0 {
		b.WriteString(mutedStyle.Render("No matches on this day."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("left/right changes day"))
		return b.String()
	}

	const headerRows = 5 // title, date tabs, blank, item count, blank
	const rowHeight = 4  // 3 content lines + 1 spacer
	maxRows := max(1, (height-headerRows)/rowHeight)

	start := 0
	if m.selected >= maxRows {
		start = m.selected - maxRows + 1
	}
	end := min(len(matches), start+maxRows)

	nameWidth := max(6, (width-6)/2)
	for i := start; i < end; i++ {
		match := matches[i]
		content := fmt.Sprintf(
			"%s vs %s\n%s\n%s  %s",
			truncate(match.HomeTeam.Name, nameWidth),
			truncate(match.AwayTeam.Name, nameWidth),
			match.Stage,
			string(match.Status),
			formatKickoff(match),
		)

		b.WriteString(renderMatchRow(content, i == m.selected))
		b.WriteString("\n\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderDetailContent(width, height int) string {
	center := func(s string) string {
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(s)
	}

	if m.loading && m.details.Match.ID == "" {
		return center(mutedStyle.Render("Loading match details..."))
	}

	details := m.details
	match := details.Match
	stats := details.Statistics
	statsAvailable := true
	if match.ID == "" || (m.selectedMatchID != "" && match.ID != m.selectedMatchID) {
		if selected, ok := m.selectedMatch(); ok {
			match = selected
			stats = types.MatchStatistics{}
			statsAvailable = false
		} else {
			return center(mutedStyle.Render("Select a match from the list."))
		}
	}

	if match.ID == "" {
		return center(mutedStyle.Render("Select a match from the list."))
	}

	textWidth := max(8, width-2)

	// --- Top: stage, teams (above the score), score, minute, venue ---
	var head strings.Builder
	headLine := func(s string) {
		head.WriteString(center(s))
		head.WriteString("\n")
	}

	// A scheduled match with no published team sheets shows a match-info card
	// (venue, kickoff, competition) instead of empty lineups.
	hasLineups := len(details.HomeLineup.Players) > 0 || len(details.AwayLineup.Players) > 0
	showMatchInfo := match.Status == types.StatusScheduled && !hasLineups

	stage := match.Stage
	if stage == "" {
		stage = details.League
	}
	if stage != "" {
		headLine(groupHeaderStyle.Render(strings.ToUpper(truncate(stage, textWidth))))
	}

	nameCap := max(4, textWidth/2-4)
	headLine(scoreTeamStyle.Render(truncate(match.HomeTeam.Name, nameCap)) +
		mutedStyle.Render("   vs   ") +
		scoreTeamStyle.Render(truncate(match.AwayTeam.Name, nameCap)))

	// The score scales with the pane (it's a priority element) but never at the
	// cost of the timeline: bestScore steps down from 2x art to 1x to a single
	// line so the rest of the head plus a compacted timeline always fits.
	stageLines := 0
	if stage != "" {
		stageLines = 1
	}
	headOther := stageLines + 1 /*teams*/ + 1 /*status*/ + 1 /*sep*/
	// Before kickoff there is no score, so showing 0-0 is misleading. Surface the
	// kickoff time as the centerpiece instead, with the date on the status line.
	if match.Status == types.StatusScheduled {
		headLine(bigScoreStyle.Render(formatKickoffTime(match)))
	} else {
		headLine(bestScore(match.Score.Home, match.Score.Away, textWidth, height, headOther, len(details.Events)))
	}
	headLine(statusBadge(match.Status) + mutedStyle.Render("  "+matchStatusDetail(match)))

	if !statsAvailable {
		if m.loading {
			head.WriteString("\n")
			headLine(mutedStyle.Render("Loading details..."))
		}
		return strings.TrimRight(head.String(), "\n")
	}

	// --- Middle ---
	// Before kickoff there is nothing to time or measure, so the timeline and
	// stats are replaced with the expected lineups, or match info if the team
	// sheets are not out yet.
	const gap = 2
	// Cap the two content columns so they stay close together and centered on
	// wide panes instead of drifting to opposite edges.
	colWidth := clamp((width-gap)/2, 16, 40)

	headStr := strings.TrimRight(head.String(), "\n")
	headLineCount := lipgloss.Height(headStr)

	if showMatchInfo {
		return headStr + "\n\n" + renderMatchInfo(match, details, width)
	}
	if match.Status == types.StatusScheduled {
		return headStr + "\n\n" +
			renderLineupBody(details.HomeLineup, details.AwayLineup, match.HomeTeam.Name, match.AwayTeam.Name, width, colWidth, gap)
	}

	// Live / finished. The score and timeline are the priority: when the pane is
	// short the timeline collapses to one row per event (dropping the connector
	// lines) so its bottom is never clipped, and the lower-priority match-facts
	// block is only appended to the stats column when it actually fits.
	bodyBudget := -1
	if height > 0 {
		bodyBudget = max(1, height-headLineCount-2)
	}

	// L toggles the on-pitch formation in place of the timeline/stats. It falls
	// through to the stats view when no usable lineup is available.
	if m.showLineups {
		if pitch, ok := renderFormationPitch(details.HomeLineup, details.AwayLineup,
			match.HomeTeam.Name, match.AwayTeam.Name, details.Events, width, bodyBudget); ok {
			sep := "\n\n"
			if height > 0 && headLineCount+2+lipgloss.Height(pitch) > height {
				sep = "\n"
			}
			return headStr + sep + pitch
		}
	}

	compact := bodyBudget > 0 && 2*len(details.Events) > bodyBudget

	statsCol := renderStatsColumn(stats, colWidth)
	rightCol := statsCol
	if facts := renderMatchFacts(details, colWidth); facts != "" {
		needed := lipgloss.Height(statsCol) + 1 + lipgloss.Height(facts)
		if bodyBudget <= 0 || needed <= bodyBudget {
			rightCol = lipgloss.JoinVertical(lipgloss.Left, statsCol, "", facts)
		}
	}

	left := lipgloss.NewStyle().Width(colWidth).MarginRight(gap).
		Render(renderTimeline(details.Events, colWidth, compact))
	right := lipgloss.NewStyle().Width(colWidth).Render(rightCol)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	body = lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(body)

	sep := "\n\n"
	if height > 0 && headLineCount+2+lipgloss.Height(body) > height {
		sep = "\n"
	}
	return headStr + sep + body
}

// renderMatchInfo shows the key facts about an upcoming fixture (competition,
// stage, kickoff, venue, location) as a centered label/value card. It is used
// for scheduled matches whose starting lineups have not been published yet.
func renderMatchInfo(match types.Match, details types.MatchDetails, width int) string {
	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	type infoRow struct{ label, value string }
	rows := make([]infoRow, 0, 5)

	competition := details.League
	if competition == "" {
		competition = string(match.Competition)
	}
	if competition != "" {
		rows = append(rows, infoRow{"Competition", competition})
	}
	if match.Group != "" {
		rows = append(rows, infoRow{"Group", match.Group})
	} else if match.Stage != "" {
		rows = append(rows, infoRow{"Stage", match.Stage})
	}
	if !match.Kickoff.IsZero() {
		rows = append(rows, infoRow{"Kickoff", match.Kickoff.In(displayLocation).Format("Mon, Jan 2 · 15:04")})
	}

	arena, location := splitVenue(details.Venue)
	if arena != "" {
		rows = append(rows, infoRow{"Venue", arena})
	}
	if location != "" {
		rows = append(rows, infoRow{"Location", location})
	}

	if len(rows) == 0 {
		return center.Render(mutedStyle.Render("No match information available."))
	}

	labelWidth := 0
	for _, row := range rows {
		if l := len([]rune(row.label)); l > labelWidth {
			labelWidth = l
		}
	}
	valueCap := max(6, width-labelWidth-4)

	valueWidth := 0
	for i := range rows {
		rows[i].value = truncate(rows[i].value, valueCap)
		if l := len([]rune(rows[i].value)); l > valueWidth {
			valueWidth = l
		}
	}

	// Pad both columns to a fixed width so every row is the same length; that
	// keeps the labels aligned in a column once the block is centered.
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		label := mutedStyle.Render(fmt.Sprintf("%-*s", labelWidth, row.label))
		value := normalRowStyle.Render(fmt.Sprintf("%-*s", valueWidth, row.value))
		lines = append(lines, label+"  "+value)
	}

	return center.Render(groupHeaderStyle.Render("MATCH INFO")) + "\n\n" +
		center.Render(strings.Join(lines, "\n")) + "\n\n" +
		center.Render(mutedStyle.Render("Lineups appear about an hour before kickoff."))
}

// renderMatchFacts builds a compact card of contextual facts (venue, city,
// attendance, referee) used to fill the space beside a shorter stats column.
// Rows with no data are omitted so the block never looks padded out, and it
// returns "" when nothing is known.
func renderMatchFacts(details types.MatchDetails, width int) string {
	type fact struct{ label, value string }
	facts := make([]fact, 0, 4)

	arena, location := splitVenue(details.Venue)
	if arena != "" {
		facts = append(facts, fact{"Venue", arena})
	}
	if location != "" {
		facts = append(facts, fact{"City", location})
	}
	if details.Attendance > 0 {
		facts = append(facts, fact{"Crowd", formatThousands(details.Attendance)})
	}
	if details.Referee != "" {
		facts = append(facts, fact{"Referee", details.Referee})
	}
	if len(facts) == 0 {
		return ""
	}

	labelWidth := 0
	for _, f := range facts {
		if l := len([]rune(f.label)); l > labelWidth {
			labelWidth = l
		}
	}
	valueCap := max(4, width-labelWidth-3)

	valueWidth := 0
	for i := range facts {
		facts[i].value = truncate(facts[i].value, valueCap)
		if l := len([]rune(facts[i].value)); l > valueWidth {
			valueWidth = l
		}
	}

	// Pad both columns so every row is the same length; centering equal-length
	// rows keeps the labels aligned in a column.
	lines := make([]string, 0, len(facts))
	for _, f := range facts {
		label := mutedStyle.Render(fmt.Sprintf("%-*s", labelWidth, f.label))
		value := normalRowStyle.Render(fmt.Sprintf("%-*s", valueWidth, f.value))
		lines = append(lines, label+"  "+value)
	}

	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
	return center.Render(tableHeaderStyle.Render("MATCH FACTS")) + "\n" +
		center.Render(strings.Join(lines, "\n"))
}

// formatThousands renders an integer with comma group separators (88966 ->
// "88,966").
func formatThousands(n int) string {
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// splitVenue separates a "Stadium, City" string into its arena and location.
func splitVenue(venue string) (arena, location string) {
	venue = strings.TrimSpace(venue)
	if venue == "" {
		return "", ""
	}
	if i := strings.LastIndex(venue, ", "); i >= 0 {
		return strings.TrimSpace(venue[:i]), strings.TrimSpace(venue[i+2:])
	}
	return venue, ""
}

// renderLineupBody shows the two expected team sheets side by side (home left,
// away right, matching the score header).
func renderLineupBody(home, away types.TeamLineup, homeName, awayName string, width, colWidth, gap int) string {
	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	left := lipgloss.NewStyle().Width(colWidth).MarginRight(gap).Render(renderLineupColumn(home, homeName, colWidth))
	right := lipgloss.NewStyle().Width(colWidth).Render(renderLineupColumn(away, awayName, colWidth))
	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	columns = lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(columns)

	return center.Render(groupHeaderStyle.Render("EXPECTED LINEUPS")) + "\n\n" + columns
}

// renderLineupColumn renders a single team sheet: team name, formation, then the
// starting XI, all centered within the column. Substitutes are omitted.
func renderLineupColumn(lineup types.TeamLineup, teamName string, width int) string {
	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	if len(lineup.Players) == 0 {
		return center.Render(scoreTeamStyle.Render(truncate(teamName, width))) + "\n" +
			center.Render(mutedStyle.Render("Not announced"))
	}

	var b strings.Builder
	b.WriteString(center.Render(scoreTeamStyle.Render(truncate(teamName, width))))
	b.WriteString("\n")
	if lineup.Formation != "" {
		b.WriteString(center.Render(tableHeaderStyle.Render(lineup.Formation)))
		b.WriteString("\n")
	}

	lines := make([]string, 0, len(lineup.Players))
	for _, player := range lineup.Players {
		if !player.Starter {
			continue
		}
		lines = append(lines, formatLineupPlayer(player, width))
	}

	b.WriteString(center.Render(strings.Join(lines, "\n")))
	return b.String()
}

// formatLineupPlayer builds a "07 R. Jiménez F" style row, shrinking the name to
// fit and dropping the redundant "SUB" position label for substitutes.
func formatLineupPlayer(player types.LineupPlayer, width int) string {
	number := "  "
	if player.Number > 0 {
		number = fmt.Sprintf("%2d", player.Number)
	}

	position := player.Position
	if !player.Starter || strings.EqualFold(position, "SUB") {
		position = ""
	}

	nameCap := max(3, width-len([]rune(number))-len([]rune(position))-2)
	name := fitName(player.Name, nameCap)

	row := number + " " + name
	if position != "" {
		row += " " + position
	}
	return row
}

// renderFormationPitch draws both starting XIs on a single vertical pitch: the
// away side at the top (attacking downward) and the home side at the bottom
// (attacking up toward the halfway line), mirroring how a match is shown on TV.
// It returns ok=false when either lineup can't be resolved into a valid
// formation, so the caller can fall back to the stats view.
func renderFormationPitch(home, away types.TeamLineup, homeName, awayName string, events []types.MatchEvent, width, maxHeight int) (string, bool) {
	inner := clamp(width-4, 22, 60)
	if width-4 < 22 {
		return "", false // too narrow to lay out players legibly
	}

	homeGK, homeRows, homeOK := arrangeFormation(home)
	awayGK, awayRows, awayOK := arrangeFormation(away)
	if !homeOK || !awayOK {
		return "", false
	}

	homeTally := tallyContributions(events, true)
	awayTally := tallyContributions(events, false)

	label := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center)
	teamLabel := func(name, formation string) string {
		s := scoreTeamStyle.Render(truncate(name, inner))
		if formation != "" {
			s += "  " + tableHeaderStyle.Render(formation)
		}
		return label.Render(s)
	}

	// Top (away) reads goalkeeper -> defence -> attack; bottom (home) reads
	// attack -> defence -> goalkeeper so both teams attack the centre line.
	// rowTally tracks which side each row belongs to for goal/assist lookups.
	var rows [][]types.LineupPlayer
	var rowTally []map[string]playerTally
	addRow := func(row []types.LineupPlayer, tally map[string]playerTally) {
		rows = append(rows, row)
		rowTally = append(rowTally, tally)
	}

	addRow([]types.LineupPlayer{awayGK}, awayTally)
	for _, row := range awayRows {
		addRow(row, awayTally)
	}
	half := len(rows) // halfway line sits between the two teams
	for i := len(homeRows) - 1; i >= 0; i-- {
		addRow(homeRows[i], homeTally)
	}
	addRow([]types.LineupPlayer{homeGK}, homeTally)

	// Spread rows vertically with a blank line between them when the pane is
	// tall enough; otherwise pack them tightly so nothing is clipped.
	gap := 0
	needed := 2 /*labels*/ + len(rows) + 1 /*halfway*/
	if maxHeight <= 0 || needed+(len(rows)-1) <= maxHeight {
		gap = 1
	}

	halfwayLine := pitchLineStyle.Render(strings.Repeat("╌", inner))
	if inner >= 3 {
		mid := (inner - 1) / 2
		halfwayLine = pitchLineStyle.Render(
			strings.Repeat("╌", mid) + "●" + strings.Repeat("╌", inner-mid-1))
	}

	var lines []string
	lines = append(lines, teamLabel(awayName, away.Formation))
	for i, row := range rows {
		if i == half {
			lines = append(lines, halfwayLine)
		}
		if gap == 1 && i > 0 && i != half {
			lines = append(lines, "")
		}
		lines = append(lines, pitchRow(row, rowTally[i], inner))
	}
	lines = append(lines, teamLabel(homeName, home.Formation))

	// Width includes the style's horizontal padding (1 col each side), so set it
	// to inner+2 to expose exactly inner columns and avoid wrapping the rows.
	pitch := pitchBorderStyle.Width(inner + 2).Render(strings.Join(lines, "\n"))
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(pitch), true
}

// arrangeFormation splits a starting XI into pitch rows. The formation string
// (e.g. "4-2-3-1") gives the row sizes from defence to attack; players are
// bucketed by the depth implied by their position, then ordered left-to-right
// within each row. The goalkeeper is returned separately. ok is false when the
// data doesn't add up to a clean XI (so the caller can fall back).
func arrangeFormation(lineup types.TeamLineup) (gk types.LineupPlayer, rows [][]types.LineupPlayer, ok bool) {
	counts := parseFormation(lineup.Formation)
	if len(counts) == 0 {
		return gk, nil, false
	}

	var keepers, outfield []types.LineupPlayer
	for _, p := range lineup.Players {
		if !p.Starter {
			continue
		}
		if core, _ := splitPos(p.Position); core == "G" || core == "GK" {
			keepers = append(keepers, p)
		} else {
			outfield = append(outfield, p)
		}
	}

	total := 0
	for _, c := range counts {
		total += c
	}
	if len(keepers) != 1 || len(outfield) != total {
		return gk, nil, false
	}

	gk = keepers[0]
	sort.SliceStable(outfield, func(i, j int) bool {
		return verticalRank(outfield[i].Position) < verticalRank(outfield[j].Position)
	})

	idx := 0
	for _, c := range counts {
		row := make([]types.LineupPlayer, c)
		copy(row, outfield[idx:idx+c])
		idx += c
		sort.SliceStable(row, func(i, j int) bool {
			return horizontalRank(row[i].Position) < horizontalRank(row[j].Position)
		})
		rows = append(rows, row)
	}
	return gk, rows, true
}

// parseFormation turns "4-2-3-1" into [4 2 3 1], ignoring empty/zero segments.
func parseFormation(formation string) []int {
	if formation == "" {
		return nil
	}
	var counts []int
	for _, part := range strings.Split(formation, "-") {
		if n := atoiTUI(strings.TrimSpace(part)); n > 0 {
			counts = append(counts, n)
		}
	}
	return counts
}

// splitPos breaks an ESPN position code like "CD-L" into its core ("CD") and
// side ("L"). Codes without a side return an empty side.
func splitPos(pos string) (core, side string) {
	p := strings.ToUpper(strings.TrimSpace(pos))
	if i := strings.IndexByte(p, '-'); i >= 0 {
		return p[:i], p[i+1:]
	}
	return p, ""
}

// verticalRank scores a position by how far up the pitch it sits (0 = keeper,
// rising through defence, midfield and attack), used to bucket players into the
// formation's rows from the back.
func verticalRank(pos string) float64 {
	core, _ := splitPos(pos)
	switch core {
	case "G", "GK":
		return 0
	case "DM", "CDM", "LDM", "RDM":
		return 2
	case "AM", "CAM":
		return 4
	case "LW", "RW", "W", "LF", "RF":
		return 4.5
	case "F", "CF", "ST", "S", "SS", "FW":
		return 5
	case "CB", "CD", "SW", "LB", "RB", "LWB", "RWB", "WB", "LCB", "RCB":
		return 1
	default:
		// Remaining central/wide midfielders (CM, LM, RM, M, ...).
		if strings.HasPrefix(core, "D") {
			return 1 // unknown defender code
		}
		return 3
	}
}

// horizontalRank scores a position from left (low) to right (high) using the
// position's side suffix and any L/R prefix, so each row reads naturally.
func horizontalRank(pos string) float64 {
	core, side := splitPos(pos)
	base := 2.0 // central
	switch {
	case strings.HasPrefix(core, "L"):
		base = 0.5
	case strings.HasPrefix(core, "R"):
		base = 3.5
	}
	switch side {
	case "L":
		base -= 1
	case "LC":
		base -= 0.5
	case "RC":
		base += 0.5
	case "R":
		base += 1
	}
	return base
}

// pitchRow renders one line of evenly spaced player tokens spanning inner cols.
func pitchRow(players []types.LineupPlayer, tally map[string]playerTally, inner int) string {
	n := len(players)
	if n == 0 {
		return strings.Repeat(" ", inner)
	}
	var b strings.Builder
	used := 0
	for i, p := range players {
		w := inner / n
		if i == n-1 {
			w = inner - used
		}
		used += w
		b.WriteString(pitchToken(p, tally[normalizeName(p.Name)], w))
	}
	return b.String()
}

// pitchToken centres a single player (accent number + short name, plus goal and
// assist icons) in cellW cols, degrading to just the number then dropping the
// name when the cell is too narrow.
func pitchToken(p types.LineupPlayer, t playerTally, cellW int) string {
	cell := lipgloss.NewStyle().Width(cellW).Align(lipgloss.Center)

	num := ""
	if p.Number > 0 {
		num = fmt.Sprintf("%d", p.Number)
	}

	suffix := ""
	if icons := contribIcons(t); icons != "" {
		suffix = " " + icons
	}

	nameMax := cellW - 2 - len(num) - 1 - lipgloss.Width(suffix)
	name := ""
	if nameMax >= 1 {
		name = truncate(lastName(p.Name), nameMax)
	}

	token := pitchNumberStyle.Render(num)
	if name != "" {
		if num != "" {
			token += " "
		}
		token += pitchNameStyle.Render(name)
	}
	token += suffix
	return cell.Render(token)
}

// playerTally counts a player's goals and assists in a match.
type playerTally struct {
	goals   int
	assists int
}

// tallyContributions counts goals and assists per player for one side. Own goals
// are excluded so a player is never credited a goal scored into their own net.
func tallyContributions(events []types.MatchEvent, home bool) map[string]playerTally {
	tally := make(map[string]playerTally)
	for _, e := range events {
		if e.Home != home {
			continue
		}
		if e.Type != types.EventGoal && e.Type != types.EventPenalty {
			continue
		}
		if e.Player != "" {
			key := normalizeName(e.Player)
			t := tally[key]
			t.goals++
			tally[key] = t
		}
		if e.Assist != "" {
			key := normalizeName(e.Assist)
			t := tally[key]
			t.assists++
			tally[key] = t
		}
	}
	return tally
}

// contribIcons renders a player's goals as ⚽ and assists as 👟, with a ×N
// multiplier when there is more than one (e.g. "⚽×2 👟").
func contribIcons(t playerTally) string {
	parts := make([]string, 0, 2)
	if t.goals > 0 {
		s := "⚽"
		if t.goals > 1 {
			s += fmt.Sprintf("×%d", t.goals)
		}
		parts = append(parts, s)
	}
	if t.assists > 0 {
		s := "👟"
		if t.assists > 1 {
			s += fmt.Sprintf("×%d", t.assists)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

// normalizeName lower-cases and trims a player name so timeline and lineup
// spellings match when tallying contributions.
func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// lastName returns the final word of a full name (e.g. "Mohamed Toure" ->
// "Toure"), which reads best in the tight space of a pitch token.
func lastName(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return name
	}
	return fields[len(fields)-1]
}

// atoiTUI is a small local int parser to avoid importing strconv here.
func atoiTUI(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// renderTimeline lays out goals and cards as a two-sided vertical timeline:
// a central axis with home-team (left team) events to the left and away-team
// (right team) events to the right, oldest at the top. When compact is set the
// connector lines between events are dropped, halving the height so every event
// stays visible in a short pane.
func renderTimeline(events []types.MatchEvent, width int, compact bool) string {
	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
	header := center.Render(tableHeaderStyle.Render("TIMELINE"))

	if len(events) == 0 {
		return header + "\n" + center.Render(mutedStyle.Render("No events yet."))
	}

	ordered := make([]types.MatchEvent, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool {
		return minuteValue(ordered[i].Minute) < minuteValue(ordered[j].Minute)
	})

	half := max(6, (width-1)/2)
	leftCell := lipgloss.NewStyle().Width(half).Align(lipgloss.Right)
	rightCell := lipgloss.NewStyle().Width(half).Align(lipgloss.Left)

	connector := lipgloss.JoinHorizontal(lipgloss.Top,
		leftCell.Render(""),
		timelineConnectorStyle.Render("│"),
		rightCell.Render(""),
	)

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	for i, event := range ordered {
		marker, markerStyle := timelineMarker(event.Type)
		label := normalRowStyle.Render(eventLabel(event, half-1))

		leftText, rightText := "", ""
		if event.Home {
			leftText = label + " "
		} else {
			rightText = " " + label
		}

		node := lipgloss.JoinHorizontal(lipgloss.Top,
			leftCell.Render(leftText),
			markerStyle.Render(marker),
			rightCell.Render(rightText),
		)
		b.WriteString(node)
		b.WriteString("\n")
		if !compact && i < len(ordered)-1 {
			b.WriteString(connector)
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// eventLabel formats a timeline entry, keeping the minute pinned next to the
// central axis (trailing for home, leading for away) and shrinking only the
// player name so the minute always stays visible within cap runes.
func eventLabel(event types.MatchEvent, cap int) string {
	player := event.Player
	if player == "" {
		player = event.Text
	}

	suffix := ""
	switch event.Type {
	case types.EventPenalty:
		suffix = " (pen)"
	case types.EventOwnGoal:
		suffix = " (OG)"
	}

	avail := max(3, cap-len([]rune(event.Minute))-2)
	nameBudget := avail - len([]rune(suffix))
	if nameBudget < 3 {
		suffix = ""
		nameBudget = avail
	}

	label := fitName(player, nameBudget) + suffix
	if event.Home {
		return label + "  " + event.Minute
	}
	return event.Minute + "  " + label
}

// fitName shrinks a player name to at most max runes, degrading gracefully:
// full name, then "F. Last Name", then the last name alone, then a truncated
// last name as a final fallback.
func fitName(name string, max int) string {
	if max <= 0 {
		return ""
	}
	if len([]rune(name)) <= max {
		return name
	}

	parts := strings.Fields(name)
	if len(parts) >= 2 {
		last := strings.Join(parts[1:], " ")
		abbrev := string([]rune(parts[0])[:1]) + ". " + last
		if len([]rune(abbrev)) <= max {
			return abbrev
		}
		if len([]rune(last)) <= max {
			return last
		}
		return truncate(last, max)
	}

	return truncate(name, max)
}

func timelineMarker(t types.MatchEventType) (string, lipgloss.Style) {
	switch t {
	case types.EventYellow:
		return "▪", yellowCardStyle
	case types.EventRed:
		return "▪", redCardStyle
	default:
		return "●", goalDotStyle
	}
}

// minuteValue extracts a sortable minute from clocks like "73'" or "90'+4'".
func minuteValue(clock string) int {
	base, added, inAdded := 0, 0, false
	for _, r := range clock {
		switch {
		case r >= '0' && r <= '9':
			if inAdded {
				added = added*10 + int(r-'0')
			} else {
				base = base*10 + int(r-'0')
			}
		case r == '+':
			inAdded = true
		}
	}
	return base*100 + added
}

func renderStatsColumn(stats types.MatchStatistics, width int) string {
	const valueWidth = 5
	// Fit the longest full label ("Possession") so nothing is truncated, but
	// shrink the column on narrow panes so the bar always keeps a usable width.
	labelWidth := clamp(width-2*valueWidth-8, 6, 10)
	barWidth := max(6, min(18, width-labelWidth-2*valueWidth-4))
	blockWidth := labelWidth + valueWidth + 1 + barWidth + 1 + valueWidth

	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
	header := lipgloss.NewStyle().Width(blockWidth).Align(lipgloss.Center).
		Render(tableHeaderStyle.Render("STATISTICS"))

	rows := []string{
		statBar("Possession", stats.PossessionHome, stats.PossessionAway, "%", barWidth, valueWidth, labelWidth, blockWidth),
		statBar("Shots", stats.ShotsHome, stats.ShotsAway, "", barWidth, valueWidth, labelWidth, blockWidth),
		statBar("On Target", stats.ShotsOnTargetHome, stats.ShotsOnTargetAway, "", barWidth, valueWidth, labelWidth, blockWidth),
		statBar("Saves", stats.SavesHome, stats.SavesAway, "", barWidth, valueWidth, labelWidth, blockWidth),
		statBar("Corners", stats.CornersHome, stats.CornersAway, "", barWidth, valueWidth, labelWidth, blockWidth),
		statBar("Offsides", stats.OffsidesHome, stats.OffsidesAway, "", barWidth, valueWidth, labelWidth, blockWidth),
		statBar("Fouls", stats.FoulsHome, stats.FoulsAway, "", barWidth, valueWidth, labelWidth, blockWidth),
		statBar("Yellows", stats.YellowCardsHome, stats.YellowCardsAway, "", barWidth, valueWidth, labelWidth, blockWidth),
		statBar("Reds", stats.RedCardsHome, stats.RedCardsAway, "", barWidth, valueWidth, labelWidth, blockWidth),
	}

	return center.Render(header) + "\n" + center.Render(strings.Join(rows, "\n"))
}

const bigDigitHeight = 5

// bigDigits holds clean, outlined ASCII block art for each numeral, used to
// render an oversized scoreline. Outlined glyphs read far better than solid
// fills at this size.
var bigDigits = map[rune][]string{
	'0': {"█████", "█   █", "█   █", "█   █", "█████"},
	'1': {"  ██ ", " ███ ", "  ██ ", "  ██ ", " ████"},
	'2': {"█████", "    █", "█████", "█    ", "█████"},
	'3': {"█████", "    █", " ████", "    █", "█████"},
	'4': {"█   █", "█   █", "█████", "    █", "    █"},
	'5': {"█████", "█    ", "█████", "    █", "█████"},
	'6': {"█████", "█    ", "█████", "█   █", "█████"},
	'7': {"█████", "    █", "   █ ", "  █  ", "  █  "},
	'8': {"█████", "█   █", "█████", "█   █", "█████"},
	'9': {"█████", "█   █", "█████", "    █", "█████"},
}

func bigNumber(n int) []string {
	rows := make([]string, bigDigitHeight)
	for i, ch := range fmt.Sprint(n) {
		art, ok := bigDigits[ch]
		if !ok {
			continue
		}
		for r := 0; r < bigDigitHeight; r++ {
			if i > 0 {
				rows[r] += " "
			}
			rows[r] += art[r]
		}
	}
	return rows
}

// bigScore lays the two scores side by side with a dash between them, scaled up
// by the given factor (1 = the base 5-row glyphs, 2 = double size, etc.).
func bigScore(home, away, scale int) string {
	if scale < 1 {
		scale = 1
	}
	left := scaleArt(bigNumber(home), scale, scale)
	right := scaleArt(bigNumber(away), scale, scale)

	h := len(left)
	dash := strings.Repeat("─", 3*scale)
	blank := strings.Repeat(" ", 3*scale)
	mid := h / 2

	lines := make([]string, h)
	for r := 0; r < h; r++ {
		sep := blank
		if r == mid {
			sep = dash
		}
		lines[r] = left[r] + "   " + sep + "   " + right[r]
	}
	return strings.Join(lines, "\n")
}

// scaleArt enlarges block art by repeating each cell sx times across and each
// row sy times down, so the same glyphs can render at multiple sizes.
func scaleArt(lines []string, sx, sy int) []string {
	if sx <= 1 && sy <= 1 {
		return lines
	}
	out := make([]string, 0, len(lines)*sy)
	for _, line := range lines {
		var b strings.Builder
		for _, r := range line {
			for i := 0; i < sx; i++ {
				b.WriteRune(r)
			}
		}
		row := b.String()
		for i := 0; i < sy; i++ {
			out = append(out, row)
		}
	}
	return out
}

// bestScore renders the scoreline as large as the pane allows: it prefers 2x
// ASCII art, falls back to 1x, then a single text line, always reserving room
// for the rest of the head plus the (compacted) timeline. headOther counts the
// non-score head lines (stage, teams, status, separator); events is the number
// of timeline entries.
func bestScore(home, away, textWidth, height, headOther, events int) string {
	for _, scale := range []int{2, 1} {
		if scale == 2 && height <= 0 {
			continue
		}
		art := bigScore(home, away, scale)
		if lipgloss.Width(art) > textWidth {
			continue
		}
		if height <= 0 || headOther+lipgloss.Height(art)+1+events <= height {
			return bigScoreStyle.Render(art)
		}
	}
	return bigScoreStyle.Render(fmt.Sprintf("%d  —  %d", home, away))
}

func matchStatusDetail(match types.Match) string {
	switch match.Status {
	case types.StatusScheduled:
		if d := formatKickoffDate(match); d != "" {
			return d
		}
		return formatKickoff(match)
	case types.StatusHalfTime:
		return "Half-time"
	case types.StatusFullTime:
		return "Full-time"
	default:
		return formatMinute(match)
	}
}

func (m Model) renderStandings() string {
	var b strings.Builder

	if len(m.standings) == 0 {
		b.WriteString(mutedStyle.Render("No standings available."))
		return b.String()
	}

	// Render each group as a bordered card and tile them four per row. Cards
	// give the twelve groups clear visual separation, a single muted column
	// header keeps the numbers calm, and the top-two (qualifying) places are
	// highlighted so the table reads at a glance instead of as a wall of zeros.
	const columns = 4
	nameWidth := clamp(max(40, m.width)/columns-19, 8, 16)

	for start := 0; start < len(m.standings); start += columns {
		end := min(len(m.standings), start+columns)
		cells := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			cells = append(cells, groupCardStyle.Render(renderStandingGroup(m.standings[i], nameWidth)))
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderStandingGroup(group types.GroupStanding, nameWidth int) string {
	// Sort by table position so rows read 1..4 (ESPN does not return them
	// pre-ordered), which keeps the highlighted top-two places at the top.
	rows := make([]types.StandingRow, len(group.Rows))
	copy(rows, group.Rows)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Position < rows[j].Position })

	var b strings.Builder
	b.WriteString(groupTitleStyle.Render(group.Group))
	b.WriteString("\n")
	b.WriteString(groupHeaderStyle.Render(fmt.Sprintf(
		"%-2s %-*s %2s %3s %3s", "#", nameWidth, "Team", "P", "GD", "Pts",
	)))
	b.WriteString("\n")
	for _, row := range rows {
		line := fmt.Sprintf(
			"%-2d %-*s %2d %3d %3d",
			row.Position,
			nameWidth, truncate(row.Team.Name, nameWidth),
			row.Played,
			row.GoalDifference,
			row.Points,
		)
		// Highlight the top two places that advance from the group.
		if row.Position <= 2 {
			b.WriteString(selectedRowStyle.Render(line))
		} else {
			b.WriteString(normalRowStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m Model) currentViewLabel() string {
	if m.currentView == ViewStandings {
		return "group standings"
	}
	return "dashboard"
}

// matchesForSelectedDate returns the matches the model is currently holding.
// The scoreboard is fetched per day, so m.matches already corresponds to the
// selected date; no extra client-side date filtering is needed (and filtering
// here would wrongly drop fixtures that kick off after midnight UTC).
func (m Model) matchesForSelectedDate() []types.Match {
	return m.matches
}

func sectionTitle(label string, width int) string {
	lineWidth := max(4, width-len(label)-1)
	return tableHeaderStyle.Render(label + " " + strings.Repeat("/", lineWidth))
}

func renderDateTabs(selected time.Time) string {
	previous := mutedStyle.Render(dayLabel(selected.AddDate(0, 0, -1)))
	current := titleStyle.Render(dayLabel(selected))
	next := mutedStyle.Render(dayLabel(selected.AddDate(0, 0, 1)))
	return lipgloss.JoinHorizontal(lipgloss.Center, previous, "   ", current, "   ", next)
}

func dayLabel(day time.Time) string {
	today := startOfDay(time.Now())
	date := startOfDay(day)
	switch {
	case date.Equal(today):
		return "Today"
	case date.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	case date.Equal(today.AddDate(0, 0, 1)):
		return "Tomorrow"
	default:
		return date.Format("Mon 02 Jan")
	}
}

func statBar(label string, home, away int, suffix string, barWidth, valueWidth, labelWidth, blockWidth int) string {
	homeSeg, awaySeg := statBarSegments(home, away, barWidth)
	bar := lipgloss.NewStyle().Width(barWidth).Render(
		tableHeaderStyle.Render(strings.Repeat("=", homeSeg)) +
			mutedStyle.Render(strings.Repeat("-", awaySeg)),
	)

	labelCol := lipgloss.NewStyle().Width(labelWidth).Align(lipgloss.Right).
		Render(mutedStyle.Render(truncate(label, labelWidth)))
	homeVal := lipgloss.NewStyle().Width(valueWidth).Align(lipgloss.Right).
		Render(fmt.Sprintf("%d%s", home, suffix))
	awayVal := lipgloss.NewStyle().Width(valueWidth).Align(lipgloss.Left).
		Render(fmt.Sprintf("%d%s", away, suffix))
	row := lipgloss.JoinHorizontal(lipgloss.Top, labelCol, homeVal, " ", bar, " ", awayVal)

	return lipgloss.NewStyle().Width(blockWidth).Align(lipgloss.Center).Render(row)
}

// statBarSegments splits a fixed-width bar between home (left) and away (right).
// Equal values always meet in the exact centre; zero totals show an empty bar.
func statBarSegments(home, away, barWidth int) (homeSeg, awaySeg int) {
	total := home + away
	if total == 0 {
		homeSeg = barWidth / 2
		return homeSeg, barWidth - homeSeg
	}
	if home == away {
		homeSeg = barWidth / 2
		return homeSeg, barWidth - homeSeg
	}
	homeSeg = (home*barWidth + total/2) / total
	if homeSeg > barWidth {
		homeSeg = barWidth
	}
	return homeSeg, barWidth - homeSeg
}

func formatMinute(match types.Match) string {
	switch match.Status {
	case types.StatusFullTime:
		return "FT"
	case types.StatusHalfTime:
		return "HT"
	case types.StatusScheduled:
		return "KO"
	default:
		if match.Minute <= 0 {
			return "-"
		}
		return fmt.Sprintf("%d'", match.Minute)
	}
}

func formatKickoff(match types.Match) string {
	if match.Kickoff.IsZero() {
		return "KO --:--"
	}
	return "KO " + match.Kickoff.In(displayLocation).Format("15:04")
}

// formatKickoffTime returns just the kickoff clock time (e.g. "15:04") in the
// display timezone, used as the centerpiece for upcoming matches in place of a
// 0-0 score.
func formatKickoffTime(match types.Match) string {
	if match.Kickoff.IsZero() {
		return "--:--"
	}
	return match.Kickoff.In(displayLocation).Format("15:04")
}

// formatKickoffDate returns the kickoff day (e.g. "Mon, Jan 2") in the display
// timezone, or an empty string when the kickoff is unknown.
func formatKickoffDate(match types.Match) string {
	if match.Kickoff.IsZero() {
		return ""
	}
	return match.Kickoff.In(displayLocation).Format("Mon, Jan 2")
}

func clamp(value, low, high int) int {
	return max(low, min(high, value))
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 0 {
		return ""
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

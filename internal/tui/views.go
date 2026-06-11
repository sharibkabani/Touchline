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

// renderDashboardBody lays out the live match list and detail panes. It returns
// only the panes (no banner/footer) so the caller can center the whole view.
func (m Model) renderDashboardBody(availableRows int) string {
	contentWidth := max(32, m.width-2)
	const paneFrameWidth = 4

	if contentWidth < 88 {
		paneInnerWidth := max(20, contentWidth-paneFrameWidth)
		// Stack the panes but keep both boxes the same height so they read as a
		// matching pair just like the side-by-side layout.
		paneHeight := max(5, (availableRows-2)/2)
		list := paneStyle.Width(paneInnerWidth).Height(paneHeight).
			Render(clampHeight(m.renderMatchListPane(paneInnerWidth, paneHeight), paneHeight))
		details := paneStyle.Width(paneInnerWidth).Height(paneHeight).
			Render(clampHeight(m.renderDetailContent(paneInnerWidth, paneHeight), paneHeight))
		return lipgloss.JoinVertical(lipgloss.Center, list, details)
	}

	leftOuterWidth := min(42, max(30, contentWidth/3))
	rightOuterWidth := max(36, contentWidth-leftOuterWidth-1)
	leftInnerWidth := max(20, leftOuterWidth-paneFrameWidth)
	rightInnerWidth := max(20, rightOuterWidth-paneFrameWidth)
	// Both panes share the exact same height so the rectangles are identical.
	paneHeight := availableRows
	left := paneStyle.Width(leftInnerWidth).Height(paneHeight).
		Render(clampHeight(m.renderMatchListPane(leftInnerWidth, paneHeight), paneHeight))
	right := paneStyle.Width(rightInnerWidth).Height(paneHeight).
		Render(clampHeight(m.renderDetailContent(rightInnerWidth, paneHeight), paneHeight))

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
		line := fmt.Sprintf(
			"%s %d - %d %s  %s  %s",
			match.HomeTeam.Name,
			match.Score.Home,
			match.Score.Away,
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
	headLine(bigScoreStyle.Render(bigScore(match.Score.Home, match.Score.Away)))
	headLine(statusBadge(match.Status) + mutedStyle.Render("  "+matchStatusDetail(match)))

	// Drop the venue line when the pane is too short — stats need the space more.
	const statsSectionLines = 8 // "STATISTICS" header + seven stat rows
	headLines := lipgloss.Height(strings.TrimRight(head.String(), "\n"))
	showVenue := statsAvailable && details.Venue != "" && !showMatchInfo &&
		(height <= 0 || headLines+1+statsSectionLines+1 <= height)
	if showVenue {
		headLine(mutedStyle.Render(truncate(details.Venue, textWidth)))
	}

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
	colWidth := max(16, (width-gap)/2)

	var body string
	switch {
	case showMatchInfo:
		body = renderMatchInfo(match, details, width)
	case match.Status == types.StatusScheduled:
		body = renderLineupBody(details.HomeLineup, details.AwayLineup, match.HomeTeam.Name, match.AwayTeam.Name, width, colWidth, gap)
	default:
		left := lipgloss.NewStyle().Width(colWidth).MarginRight(gap).Render(renderTimeline(details.Events, colWidth))
		right := lipgloss.NewStyle().Width(colWidth).Render(renderStatsColumn(stats, colWidth))
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	headStr := strings.TrimRight(head.String(), "\n")
	headLineCount := lipgloss.Height(headStr)
	bodySep := "\n\n"
	if height > 0 && headLineCount+1+statsSectionLines > height {
		bodySep = "\n"
	}
	return headStr + bodySep + body
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
		rows = append(rows, infoRow{"Kickoff", match.Kickoff.Local().Format("Mon, Jan 2 · 15:04")})
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

// renderTimeline lays out goals and cards as a two-sided vertical timeline:
// a central axis with home-team (left team) events to the left and away-team
// (right team) events to the right, oldest at the top.
func renderTimeline(events []types.MatchEvent, width int) string {
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
		if i < len(ordered)-1 {
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
		statBar("Corners", stats.CornersHome, stats.CornersAway, "", barWidth, valueWidth, labelWidth, blockWidth),
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

// bigScore lays the two scores side by side with a dash between them.
func bigScore(home, away int) string {
	left := bigNumber(home)
	right := bigNumber(away)
	sep := []string{"     ", "     ", " ─── ", "     ", "     "}

	lines := make([]string, bigDigitHeight)
	for r := 0; r < bigDigitHeight; r++ {
		lines[r] = left[r] + "  " + sep[r] + "  " + right[r]
	}
	return strings.Join(lines, "\n")
}

func matchStatusDetail(match types.Match) string {
	switch match.Status {
	case types.StatusScheduled:
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
	return "KO " + match.Kickoff.Local().Format("15:04")
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

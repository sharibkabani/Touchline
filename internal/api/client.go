package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"touchline/internal/types"
)

// FootballProvider is the boundary the rest of the application depends on.
//
// It is intentionally small and date-aware: the TUI pages day by day, and
// bundling per-match details inside the scoreboard means a single day costs a
// single upstream request. Any concrete provider (mock, ESPN, a future
// WebSocket feed) only has to satisfy this contract.
type FootballProvider interface {
	GetScoreboard(ctx context.Context, competition types.CompetitionCode, date time.Time) (types.Scoreboard, error)
	GetStandings(ctx context.Context, competition types.CompetitionCode) ([]types.GroupStanding, error)
}

// MockProvider serves bundled JSON so the app runs without network or API keys.
// It ignores the requested date and always returns the sample fixtures, which is
// enough for offline development and demos.
type MockProvider struct {
	dir string
}

func NewMockProvider(dir string) *MockProvider {
	return &MockProvider{dir: dir}
}

func (p *MockProvider) GetScoreboard(ctx context.Context, competition types.CompetitionCode, _ time.Time) (types.Scoreboard, error) {
	var matchesResp liveMatchesResponse
	if err := p.readJSON(ctx, "live_matches.json", &matchesResp); err != nil {
		return types.Scoreboard{}, err
	}

	var detailsResp matchDetailsResponse
	if err := p.readJSON(ctx, "match_details.json", &detailsResp); err != nil {
		return types.Scoreboard{}, err
	}

	matches := make([]types.Match, 0, len(matchesResp.Matches))
	for _, match := range matchesResp.Matches {
		if match.Competition == competition {
			matches = append(matches, match)
		}
	}

	details := make(map[string]types.MatchDetails, len(detailsResp.Details))
	for _, detail := range detailsResp.Details {
		details[detail.Match.ID] = detail
	}

	return types.Scoreboard{Matches: matches, Details: details}, nil
}

func (p *MockProvider) GetStandings(ctx context.Context, _ types.CompetitionCode) ([]types.GroupStanding, error) {
	var response standingsResponse
	if err := p.readJSON(ctx, "standings.json", &response); err != nil {
		return nil, err
	}

	return response.Groups, nil
}

func (p *MockProvider) readJSON(ctx context.Context, name string, target any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	path := filepath.Join(p.dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read mock response %s: %w", path, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode mock response %s: %w", path, err)
	}

	return nil
}

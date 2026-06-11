package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"touchline/internal/api"
	"touchline/internal/cache"
	"touchline/internal/types"
)

type MatchService struct {
	provider        api.FootballProvider
	competition     types.CompetitionCode
	scoreboardCache *cache.Cache[types.Scoreboard]
	logger          *slog.Logger
}

func NewMatchService(
	provider api.FootballProvider,
	competition types.CompetitionCode,
	scoreboardCache *cache.Cache[types.Scoreboard],
	logger *slog.Logger,
) *MatchService {
	return &MatchService{
		provider:        provider,
		competition:     competition,
		scoreboardCache: scoreboardCache,
		logger:          logger,
	}
}

// scoreboard fetches (and caches) a whole day at once. Matches and details for
// that day share this single result, so paging between matches never triggers
// extra upstream requests.
func (s *MatchService) scoreboard(ctx context.Context, date time.Time, force bool) (types.Scoreboard, error) {
	key := fmt.Sprintf("scoreboard:%s:%s", s.competition, date.Format("20060102"))
	if !force {
		if board, ok := s.scoreboardCache.Get(key); ok {
			return board, nil
		}
	}

	board, err := s.provider.GetScoreboard(ctx, s.competition, date)
	if err != nil {
		s.logger.Warn("failed to get scoreboard", "date", date.Format("2006-01-02"), "error", err)
		return types.Scoreboard{}, err
	}

	s.scoreboardCache.Set(key, board)
	return board, nil
}

func (s *MatchService) Matches(ctx context.Context, date time.Time, force bool) ([]types.Match, error) {
	board, err := s.scoreboard(ctx, date, force)
	if err != nil {
		return nil, err
	}
	return board.Matches, nil
}

func (s *MatchService) MatchDetails(ctx context.Context, date time.Time, matchID string, force bool) (types.MatchDetails, error) {
	board, err := s.scoreboard(ctx, date, force)
	if err != nil {
		return types.MatchDetails{}, err
	}

	details, ok := board.Details[matchID]
	if !ok {
		return types.MatchDetails{}, fmt.Errorf("no match details available for %q", matchID)
	}
	return details, nil
}

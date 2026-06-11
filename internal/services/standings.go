package services

import (
	"context"
	"fmt"
	"log/slog"

	"touchline/internal/api"
	"touchline/internal/cache"
	"touchline/internal/types"
)

type StandingService struct {
	provider    api.FootballProvider
	competition types.CompetitionCode
	cache       *cache.Cache[[]types.GroupStanding]
	logger      *slog.Logger
}

func NewStandingService(
	provider api.FootballProvider,
	competition types.CompetitionCode,
	cache *cache.Cache[[]types.GroupStanding],
	logger *slog.Logger,
) *StandingService {
	return &StandingService{
		provider:    provider,
		competition: competition,
		cache:       cache,
		logger:      logger,
	}
}

func (s *StandingService) Standings(ctx context.Context, force bool) ([]types.GroupStanding, error) {
	key := fmt.Sprintf("standings:%s", s.competition)
	if !force {
		if standings, ok := s.cache.Get(key); ok {
			return standings, nil
		}
	}

	standings, err := s.provider.GetStandings(ctx, s.competition)
	if err != nil {
		s.logger.Warn("failed to get standings", "error", err)
		return nil, err
	}

	s.cache.Set(key, standings)
	return standings, nil
}

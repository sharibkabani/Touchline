package services

import (
	"context"
	"fmt"
	"log/slog"

	"touchline/internal/api"
	"touchline/internal/cache"
	"touchline/internal/types"
)

type BracketService struct {
	provider    api.FootballProvider
	competition types.CompetitionCode
	cache       *cache.Cache[[]types.BracketRound]
	logger      *slog.Logger
}

func NewBracketService(
	provider api.FootballProvider,
	competition types.CompetitionCode,
	cache *cache.Cache[[]types.BracketRound],
	logger *slog.Logger,
) *BracketService {
	return &BracketService{
		provider:    provider,
		competition: competition,
		cache:       cache,
		logger:      logger,
	}
}

func (s *BracketService) Bracket(ctx context.Context, force bool) ([]types.BracketRound, error) {
	key := fmt.Sprintf("bracket:%s", s.competition)
	if !force {
		if bracket, ok := s.cache.Get(key); ok {
			return bracket, nil
		}
	}

	bracket, err := s.provider.GetBracket(ctx, s.competition)
	if err != nil {
		s.logger.Warn("failed to get bracket", "error", err)
		return nil, err
	}

	s.cache.Set(key, bracket)
	return bracket, nil
}

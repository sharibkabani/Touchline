package api

import "touchline/internal/types"

type liveMatchesResponse struct {
	Matches []types.Match `json:"matches"`
}

type matchDetailsResponse struct {
	Details []types.MatchDetails `json:"details"`
}

type standingsResponse struct {
	Groups []types.GroupStanding `json:"groups"`
}

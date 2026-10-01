package search

import (
	"context"
	"strconv"

	osclient "search/internal/opensearch"
)

type Service struct {
	os *osclient.Client
}

func NewService(os *osclient.Client) *Service {
	return &Service{os: os}
}

func (s *Service) SearchMessages(ctx context.Context, query string, chatID int64, cursor string, limit int32) ([]osclient.Hit, string, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	from := 0
	if cursor != "" {
		from, _ = strconv.Atoi(cursor)
	}
	hits, err := s.os.SearchMessages(ctx, query, chatID, from, int(limit))
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(hits) == int(limit) {
		next = strconv.Itoa(from + int(limit))
	}
	return hits, next, nil
}

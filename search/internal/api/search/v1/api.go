package searchv1api

import (
	"context"
	"strconv"

	searchv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/search/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchsvc "search/internal/service/search"
)

type ctxKeyUserID struct{}

func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID{}, id)
}

func ParseUserIDString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

type Implementation struct {
	searchv1.UnimplementedSearchServiceServer
	svc *searchsvc.Service
}

func New(svc *searchsvc.Service) *Implementation {
	return &Implementation{svc: svc}
}

func (i *Implementation) SearchMessages(ctx context.Context, req *searchv1.SearchMessagesRequest) (*searchv1.SearchMessagesResponse, error) {
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, "query required")
	}
	hits, next, err := i.svc.SearchMessages(ctx, req.GetQuery(), req.GetChatId(), req.GetCursor(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	out := make([]*searchv1.SearchMessageHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, &searchv1.SearchMessageHit{
			MessageId: h.MessageID,
			ChatId:    h.ChatID,
			SenderId:  h.SenderID,
			Text:      h.Text,
			Score:     h.Score,
		})
	}
	return &searchv1.SearchMessagesResponse{Hits: out, NextCursor: next}, nil
}

func (i *Implementation) SearchChats(ctx context.Context, req *searchv1.SearchChatsRequest) (*searchv1.SearchChatsResponse, error) {
	// Chat name search not indexed yet — return empty.
	return &searchv1.SearchChatsResponse{}, nil
}

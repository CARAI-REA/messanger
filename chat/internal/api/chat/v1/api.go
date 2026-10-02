package chatv1api

import (
	"context"
	"strconv"
	"strings"

	chatv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/chat/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"chat/internal/model"
	"chat/internal/service"
)

type ctxKeyUserID struct{}

func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID{}, id)
}

func ParseUserIDString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func userIDFromCtx(ctx context.Context) (int64, error) {
	v := ctx.Value(ctxKeyUserID{})
	if v == nil {
		return 0, status.Error(codes.Unauthenticated, "missing user")
	}
	id, ok := v.(int64)
	if !ok || id == 0 {
		return 0, status.Error(codes.Unauthenticated, "invalid user")
	}
	return id, nil
}

type Implementation struct {
	chatv1.UnimplementedChatServiceServer
	svc service.ChatService
}

func NewImplementation(svc service.ChatService) *Implementation {
	return &Implementation{svc: svc}
}

func (i *Implementation) CreateChat(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	info := req.GetChatInfo()
	if info == nil {
		return nil, status.Error(codes.InvalidArgument, "chat_info required")
	}
	id, err := i.svc.CreateChat(ctx, uid, info.Name, info.Description, info.UserIds)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &chatv1.CreateChatResponse{ChatId: id}, nil
}

func (i *Implementation) GetOrCreateDirectChat(ctx context.Context, req *chatv1.GetOrCreateDirectChatRequest) (*chatv1.CreateChatResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	id, err := i.svc.GetOrCreateDirectChat(ctx, uid, req.GetPeerUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return &chatv1.CreateChatResponse{ChatId: id}, nil
}

func (i *Implementation) UpdateChat(ctx context.Context, req *chatv1.UpdateChatRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	var name, desc, avatar *string
	if req.Name != nil {
		v := req.Name.GetValue()
		name = &v
	}
	if req.Description != nil {
		v := req.Description.GetValue()
		desc = &v
	}
	if req.AvatarFileId != nil {
		v := req.AvatarFileId.GetValue()
		avatar = &v
	}
	if err := i.svc.UpdateChat(ctx, uid, req.GetChatId(), name, desc, avatar); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) DeleteChat(ctx context.Context, req *chatv1.DeleteChatRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.DeleteChat(ctx, uid, req.GetChatId()); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) GetChat(ctx context.Context, req *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	c, members, role, err := i.svc.GetChat(ctx, uid, req.GetChatId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	return &chatv1.GetChatResponse{
		Chat:           toChatProto(c),
		ParticipantIds: members,
		MyRole:         chatv1.Role(role),
	}, nil
}

func (i *Implementation) ListChats(ctx context.Context, req *chatv1.ListChatsRequest) (*chatv1.ListChatsResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	chats, next, err := i.svc.ListChats(ctx, uid, req.GetCursor(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	out := make([]*chatv1.Chat, 0, len(chats))
	for i := range chats {
		out = append(out, toChatProto(&chats[i]))
	}
	return &chatv1.ListChatsResponse{Chats: out, NextCursor: next}, nil
}

func (i *Implementation) ListChatIDs(ctx context.Context, _ *chatv1.ListChatIDsRequest) (*chatv1.ListChatIDsResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := i.svc.ListChatIDs(ctx, uid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &chatv1.ListChatIDsResponse{ChatIds: ids}, nil
}

func (i *Implementation) AddUser(ctx context.Context, req *chatv1.AddUserRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.AddUser(ctx, uid, req.GetChatId(), req.GetUserId(), int32(req.GetRole())); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) RemoveUser(ctx context.Context, req *chatv1.RemoveUserRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.RemoveUser(ctx, uid, req.GetChatId(), req.GetUserId()); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) UpdateUserRole(ctx context.Context, req *chatv1.UpdateUserRoleRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.UpdateUserRole(ctx, uid, req.GetChatId(), req.GetUserId(), int32(req.GetRole())); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) SendMessage(ctx context.Context, req *chatv1.SendMessageRequest) (*chatv1.SendMessageResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	id, err := i.svc.SendMessage(ctx, uid, req.GetChatId(), req.GetText(), req.GetIdempotencyKey(), req.GetAttachmentIds(), req.GetReplyToMessageId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &chatv1.SendMessageResponse{MessageId: id}, nil
}

func (i *Implementation) EditMessage(ctx context.Context, req *chatv1.EditMessageRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.EditMessage(ctx, uid, req.GetMessageId(), req.GetText()); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) DeleteMessage(ctx context.Context, req *chatv1.DeleteMessageRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.DeleteMessage(ctx, uid, req.GetMessageId()); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) PinMessage(ctx context.Context, req *chatv1.PinMessageRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.PinMessage(ctx, uid, req.GetMessageId(), req.GetIsPinned()); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) ListMessages(ctx context.Context, req *chatv1.ListMessagesRequest) (*chatv1.ListMessagesResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	msgs, more, err := i.svc.ListMessages(ctx, uid, req.GetChatId(), req.GetBeforeId(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	out := make([]*chatv1.Message, 0, len(msgs))
	for i := range msgs {
		out = append(out, toMessageProto(&msgs[i]))
	}
	return &chatv1.ListMessagesResponse{Messages: out, HasMore: more}, nil
}

func (i *Implementation) MarkRead(ctx context.Context, req *chatv1.MarkReadRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.MarkRead(ctx, uid, req.GetChatId(), req.GetMessageId()); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) PinChat(ctx context.Context, req *chatv1.PinChatRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.PinChat(ctx, uid, req.GetChatId(), req.GetIsPinned()); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) GetUnreadCounts(ctx context.Context, req *chatv1.GetUnreadCountsRequest) (*chatv1.GetUnreadCountsResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := i.svc.GetUnreadCounts(ctx, uid, req.GetChatIds())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &chatv1.GetUnreadCountsResponse{Counts: counts}, nil
}

func toChatProto(c *model.Chat) *chatv1.Chat {
	ch := &chatv1.Chat{
		ChatId:  c.ID,
		OwnerId: c.OwnerID,
		ChatInfo: &chatv1.ChatInfo{
			Name:        c.Name,
			Description: c.Description,
			UserIds:     c.ParticipantIDs,
		},
		UnreadCount:        c.UnreadCount,
		ChatType:           chatv1.ChatType(c.ChatType),
		LastMessagePreview: c.LastMessagePreview,
		ParticipantIds:     c.ParticipantIDs,
		PeerUserId:         c.PeerUserID,
		AvatarFileId:       c.AvatarFileID,
		IsPinned:           c.IsPinned,
	}
	if c.LastMessageAt != nil {
		ch.LastMessageAt = timestamppb.New(*c.LastMessageAt)
	}
	if c.LastMessageID != nil {
		ch.LastMessageId = *c.LastMessageID
	}
	return ch
}

func toMessageProto(m *model.Message) *chatv1.Message {
	return &chatv1.Message{
		MessageId:        m.ID,
		SenderId:         m.SenderID,
		ChatId:           m.ChatID,
		Text:             m.Text,
		IsPinned:         m.IsPinned,
		SendAt:           timestamppb.New(m.SendAt),
		UpdatedAt:        timestamppb.New(m.UpdatedAt),
		AttachmentIds:    m.AttachmentIDs,
		ReplyToMessageId: m.ReplyToMessageID,
	}
}

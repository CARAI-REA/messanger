package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	eventsv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/events/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"chat/internal/config"
	"chat/internal/model"
	"chat/internal/repository"
	"chat/internal/service"
)

type serv struct {
	repo repository.ChatRepository
}

func NewService(repo repository.ChatRepository, _ repository.OutboxRepository) service.ChatService {
	return &serv{repo: repo}
}

func newEvent(chatID, actorID int64) *eventsv1.ChatRealtimeEvent {
	return &eventsv1.ChatRealtimeEvent{
		EventId: uuid.NewString(),
		ChatId:  chatID,
		ActorId: actorID,
		At:      timestamppb.Now(),
	}
}

func (s *serv) outboxEvent(chatID int64, ev *eventsv1.ChatRealtimeEvent) (repository.OutboxEvent, error) {
	b, err := proto.Marshal(ev)
	if err != nil {
		return repository.OutboxEvent{}, err
	}
	topic := config.AppConfig().Kafka.ChatEventsTopic()
	return repository.OutboxEvent{
		Topic:   topic,
		Key:     strconv.FormatInt(chatID, 10),
		Payload: b,
	}, nil
}

func (s *serv) createChatEvents(chatID, actorID int64, memberIDs []int64) ([]repository.OutboxEvent, error) {
	seen := map[int64]struct{}{actorID: {}}
	var events []repository.OutboxEvent

	evOwner := newEvent(chatID, actorID)
	evOwner.Payload = &eventsv1.ChatRealtimeEvent_MemberChanged{
		MemberChanged: &eventsv1.MemberChanged{UserId: actorID, Action: "created", Role: 2},
	}
	oe, err := s.outboxEvent(chatID, evOwner)
	if err != nil {
		return nil, err
	}
	events = append(events, oe)

	for _, uid := range memberIDs {
		if _, ok := seen[uid]; ok || uid <= 0 {
			continue
		}
		seen[uid] = struct{}{}
		ev := newEvent(chatID, actorID)
		ev.Payload = &eventsv1.ChatRealtimeEvent_MemberChanged{
			MemberChanged: &eventsv1.MemberChanged{UserId: uid, Action: "added", Role: 0},
		}
		oe, err := s.outboxEvent(chatID, ev)
		if err != nil {
			return nil, err
		}
		events = append(events, oe)
	}
	return events, nil
}

func (s *serv) CreateChat(ctx context.Context, actorID int64, name, description string, memberIDs []int64) (int64, error) {
	name = strings.TrimSpace(name)
	others := 0
	for _, id := range memberIDs {
		if id > 0 && id != actorID {
			others++
		}
	}
	if others < 1 {
		return 0, fmt.Errorf("group requires at least one other member")
	}
	if name == "" {
		return 0, fmt.Errorf("group name required")
	}
	return s.repo.CreateChat(ctx, actorID, name, description, model.ChatTypeGroup, memberIDs, func(chatID int64) ([]repository.OutboxEvent, error) {
		return s.createChatEvents(chatID, actorID, memberIDs)
	})
}

func (s *serv) GetOrCreateDirectChat(ctx context.Context, actorID, peerUserID int64) (int64, error) {
	if peerUserID <= 0 || peerUserID == actorID {
		return 0, fmt.Errorf("invalid peer")
	}
	return s.repo.GetOrCreateDirect(ctx, actorID, peerUserID, func(chatID int64) ([]repository.OutboxEvent, error) {
		return s.createChatEvents(chatID, actorID, []int64{peerUserID})
	})
}

func (s *serv) requireAdmin(ctx context.Context, chatID, actorID int64) error {
	role, err := s.repo.GetMemberRole(ctx, chatID, actorID)
	if err != nil {
		return err
	}
	// ROLE_ADMIN=1, ROLE_OWNER=2
	if role < 1 {
		return fmt.Errorf("admin only")
	}
	return nil
}

func (s *serv) UpdateChat(ctx context.Context, actorID, chatID int64, name, description, avatarFileID *string) error {
	if err := s.requireAdmin(ctx, chatID, actorID); err != nil {
		return err
	}
	return s.repo.UpdateChat(ctx, chatID, actorID, name, description, avatarFileID)
}

func (s *serv) DeleteChat(ctx context.Context, actorID, chatID int64) error {
	return s.repo.SoftDeleteChat(ctx, chatID, actorID)
}

func (s *serv) GetChat(ctx context.Context, actorID, chatID int64) (*model.Chat, []int64, int32, error) {
	c, members, err := s.repo.GetChat(ctx, chatID, actorID)
	if err != nil {
		return nil, nil, 0, err
	}
	role, err := s.repo.GetMemberRole(ctx, chatID, actorID)
	if err != nil {
		return nil, nil, 0, err
	}
	return c, members, role, nil
}

func (s *serv) ListChats(ctx context.Context, actorID int64, cursor string, limit int32) ([]model.Chat, string, error) {
	return s.repo.ListChats(ctx, actorID, cursor, limit)
}

func (s *serv) ListChatIDs(ctx context.Context, actorID int64) ([]int64, error) {
	return s.repo.ListChatIDs(ctx, actorID)
}

func (s *serv) AddUser(ctx context.Context, actorID, chatID, userID int64, role int32) error {
	if err := s.requireAdmin(ctx, chatID, actorID); err != nil {
		return err
	}
	typ, err := s.repo.GetChatType(ctx, chatID)
	if err != nil {
		return err
	}
	if typ == model.ChatTypeDirect {
		return fmt.Errorf("cannot add members to a direct chat")
	}
	if role >= 2 {
		role = 0 // only owner exists once; new members are regular users
	}
	ev := newEvent(chatID, actorID)
	ev.Payload = &eventsv1.ChatRealtimeEvent_MemberChanged{
		MemberChanged: &eventsv1.MemberChanged{UserId: userID, Action: "added", Role: role},
	}
	oe, err := s.outboxEvent(chatID, ev)
	if err != nil {
		return err
	}
	return s.repo.AddMember(ctx, chatID, userID, role, []repository.OutboxEvent{oe})
}

func (s *serv) RemoveUser(ctx context.Context, actorID, chatID, userID int64) error {
	if err := s.requireAdmin(ctx, chatID, actorID); err != nil {
		return err
	}
	typ, err := s.repo.GetChatType(ctx, chatID)
	if err != nil {
		return err
	}
	if typ == model.ChatTypeDirect {
		return fmt.Errorf("cannot remove members from a direct chat")
	}
	targetRole, err := s.repo.GetMemberRole(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if targetRole >= 2 {
		return fmt.Errorf("cannot remove group owner")
	}
	ev := newEvent(chatID, actorID)
	ev.Payload = &eventsv1.ChatRealtimeEvent_MemberChanged{
		MemberChanged: &eventsv1.MemberChanged{UserId: userID, Action: "removed"},
	}
	oe, err := s.outboxEvent(chatID, ev)
	if err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, chatID, userID, []repository.OutboxEvent{oe})
}

func (s *serv) UpdateUserRole(ctx context.Context, actorID, chatID, userID int64, role int32) error {
	if err := s.requireAdmin(ctx, chatID, actorID); err != nil {
		return err
	}
	actorRole, err := s.repo.GetMemberRole(ctx, chatID, actorID)
	if err != nil {
		return err
	}
	if role >= 2 && actorRole < 2 {
		return fmt.Errorf("only owner can assign owner")
	}
	ev := newEvent(chatID, actorID)
	ev.Payload = &eventsv1.ChatRealtimeEvent_MemberChanged{
		MemberChanged: &eventsv1.MemberChanged{UserId: userID, Action: "role_updated", Role: role},
	}
	oe, err := s.outboxEvent(chatID, ev)
	if err != nil {
		return err
	}
	return s.repo.UpdateMemberRole(ctx, chatID, userID, role, []repository.OutboxEvent{oe})
}

func (s *serv) SendMessage(ctx context.Context, actorID, chatID int64, text, idemKey string, attachments []string, replyTo int64) (int64, error) {
	ok, err := s.repo.IsMember(ctx, chatID, actorID)
	if err != nil || !ok {
		return 0, fmt.Errorf("not a member")
	}
	m, _, err := s.repo.SendMessage(ctx, chatID, actorID, text, idemKey, attachments, replyTo, func(msg *model.Message) (*repository.OutboxEvent, error) {
		ev := newEvent(chatID, actorID)
		ev.Payload = &eventsv1.ChatRealtimeEvent_MessageCreated{
			MessageCreated: &eventsv1.MessageCreated{
				MessageId:     msg.ID,
				SenderId:      msg.SenderID,
				Text:          msg.Text,
				AttachmentIds: msg.AttachmentIDs,
				SendAt:        timestamppb.New(msg.SendAt),
			},
		}
		oe, err := s.outboxEvent(chatID, ev)
		if err != nil {
			return nil, err
		}
		return &oe, nil
	})
	if err != nil {
		return 0, err
	}
	return m.ID, nil
}

func (s *serv) EditMessage(ctx context.Context, actorID, messageID int64, text string) error {
	_, err := s.repo.EditMessage(ctx, messageID, actorID, text, func(m *model.Message) ([]repository.OutboxEvent, error) {
		ev := newEvent(m.ChatID, actorID)
		ev.Payload = &eventsv1.ChatRealtimeEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{
				MessageId: m.ID,
				Text:      m.Text,
				UpdatedAt: timestamppb.New(m.UpdatedAt),
			},
		}
		oe, err := s.outboxEvent(m.ChatID, ev)
		if err != nil {
			return nil, err
		}
		return []repository.OutboxEvent{oe}, nil
	})
	return err
}

func (s *serv) DeleteMessage(ctx context.Context, actorID, messageID int64) error {
	_, err := s.repo.DeleteMessage(ctx, messageID, actorID, func(m *model.Message) ([]repository.OutboxEvent, error) {
		ev := newEvent(m.ChatID, actorID)
		ev.Payload = &eventsv1.ChatRealtimeEvent_MessageDeleted{
			MessageDeleted: &eventsv1.MessageDeleted{MessageId: m.ID},
		}
		oe, err := s.outboxEvent(m.ChatID, ev)
		if err != nil {
			return nil, err
		}
		return []repository.OutboxEvent{oe}, nil
	})
	return err
}

func (s *serv) PinMessage(ctx context.Context, actorID, messageID int64, pinned bool) error {
	_, err := s.repo.PinMessage(ctx, messageID, actorID, pinned, func(m *model.Message) ([]repository.OutboxEvent, error) {
		ev := newEvent(m.ChatID, actorID)
		ev.Payload = &eventsv1.ChatRealtimeEvent_MessagePinned{
			MessagePinned: &eventsv1.MessagePinned{MessageId: m.ID, IsPinned: pinned},
		}
		oe, err := s.outboxEvent(m.ChatID, ev)
		if err != nil {
			return nil, err
		}
		return []repository.OutboxEvent{oe}, nil
	})
	return err
}

func (s *serv) ListMessages(ctx context.Context, actorID, chatID, beforeID int64, limit int32) ([]model.Message, bool, error) {
	return s.repo.ListMessages(ctx, chatID, actorID, beforeID, limit)
}

func (s *serv) MarkRead(ctx context.Context, actorID, chatID, messageID int64) error {
	return s.repo.MarkRead(ctx, chatID, actorID, messageID, func() ([]repository.OutboxEvent, error) {
		ev := newEvent(chatID, actorID)
		ev.Payload = &eventsv1.ChatRealtimeEvent_ReceiptRead{
			ReceiptRead: &eventsv1.ReceiptRead{UserId: actorID, MessageId: messageID},
		}
		oe, err := s.outboxEvent(chatID, ev)
		if err != nil {
			return nil, err
		}
		return []repository.OutboxEvent{oe}, nil
	})
}

func (s *serv) GetUnreadCounts(ctx context.Context, actorID int64, chatIDs []int64) (map[int64]int64, error) {
	return s.repo.GetUnreadCounts(ctx, actorID, chatIDs)
}

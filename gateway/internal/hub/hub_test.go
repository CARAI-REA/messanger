package hub

import "testing"

func TestSendToChatOnlyDeliversToMembers(t *testing.T) {
	h := New()

	memberA := &Client{UserID: 1, Send: make(chan []byte, 1), Chats: map[int64]struct{}{10: {}}}
	memberB := &Client{UserID: 2, Send: make(chan []byte, 1), Chats: map[int64]struct{}{10: {}, 20: {}}}
	nonMember := &Client{UserID: 3, Send: make(chan []byte, 1), Chats: map[int64]struct{}{20: {}}}

	h.Register(memberA)
	h.Register(memberB)
	h.Register(nonMember)

	payload := []byte(`{"type":"chat.realtime","chat_id":10}`)
	h.SendToChat(10, payload)

	assertReceived(t, memberA.Send, string(payload))
	assertReceived(t, memberB.Send, string(payload))
	assertNotReceived(t, nonMember.Send)
}

func TestAddChatHotMembership(t *testing.T) {
	h := New()
	c := &Client{UserID: 7, Send: make(chan []byte, 1), Chats: map[int64]struct{}{}}
	h.Register(c)

	payload := []byte(`{"chat":99}`)
	h.SendToChat(99, payload)
	assertNotReceived(t, c.Send)

	h.AddChat(7, 99)
	h.SendToChat(99, payload)
	assertReceived(t, c.Send, string(payload))
}

func TestRemoveChatStopsDelivery(t *testing.T) {
	h := New()
	c := &Client{UserID: 7, Send: make(chan []byte, 1), Chats: map[int64]struct{}{99: {}}}
	h.Register(c)

	h.RemoveChat(7, 99)
	h.SendToChat(99, []byte(`x`))
	assertNotReceived(t, c.Send)
}

func TestBroadcastStillDeliversToAllClients(t *testing.T) {
	h := New()

	clientA := &Client{UserID: 1, Send: make(chan []byte, 1), Chats: map[int64]struct{}{10: {}}}
	clientB := &Client{UserID: 2, Send: make(chan []byte, 1)}

	h.Register(clientA)
	h.Register(clientB)

	payload := []byte(`{"type":"presence"}`)
	h.Broadcast(payload)

	assertReceived(t, clientA.Send, string(payload))
	assertReceived(t, clientB.Send, string(payload))
}

func assertReceived(t *testing.T, ch <-chan []byte, want string) {
	t.Helper()

	select {
	case got := <-ch:
		if string(got) != want {
			t.Fatalf("unexpected payload: got %q want %q", string(got), want)
		}
	default:
		t.Fatalf("expected payload %q", want)
	}
}

func assertNotReceived(t *testing.T, ch <-chan []byte) {
	t.Helper()

	select {
	case got := <-ch:
		t.Fatalf("unexpected payload %q", string(got))
	default:
	}
}

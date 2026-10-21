package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
)

// fakeAssistantPrefs fills the user's memories and conversations once: two
// memories, and a conversation on the living room's television.
func (a *fakeAssistant) fakeAssistantPrefs() {
	if a.prefs {
		return
	}
	a.prefs = true
	at := timestamppb.New(time.Date(2026, 10, 20, 20, 30, 0, 0, time.Local))
	a.memories = []*assistantv1.AssistantMemory{
		assistantv1.AssistantMemory_builder{Id: new("m1"), Text: new("Likes science fiction"), UpdateTime: at}.Build(),
		assistantv1.AssistantMemory_builder{Id: new("m2"), Text: new("Watches with subtitles in English"), UpdateTime: at}.Build(),
	}
	a.convs = []*assistantv1.AssistantConversation{
		assistantv1.AssistantConversation_builder{Id: new("a1"), DeviceName: new("Living Room TV"), UpdateTime: at}.Build(),
	}
}

func (a *fakeAssistant) ListAssistantMemories(context.Context, *assistantv1.ListAssistantMemoriesRequest) (*assistantv1.ListAssistantMemoriesResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fakeAssistantPrefs()
	return assistantv1.ListAssistantMemoriesResponse_builder{Memories: slices.Clone(a.memories)}.Build(), nil
}

func (a *fakeAssistant) UpdateAssistantMemory(_ context.Context, r *assistantv1.UpdateAssistantMemoryRequest) (*assistantv1.UpdateAssistantMemoryResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fakeAssistantPrefs()
	m := assistantv1.AssistantMemory_builder{Id: new(fmt.Sprintf("m%d", len(a.memories)+1)), Text: new(r.GetText()), UpdateTime: timestamppb.Now()}.Build()
	a.memories = append(a.memories, m)
	return assistantv1.UpdateAssistantMemoryResponse_builder{Memory: m}.Build(), nil
}

func (a *fakeAssistant) DeleteAssistantMemories(_ context.Context, r *assistantv1.DeleteAssistantMemoriesRequest) (*assistantv1.DeleteAssistantMemoriesResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fakeAssistantPrefs()
	if len(r.GetIds()) == 0 {
		a.memories = nil
	}
	a.memories = slices.DeleteFunc(a.memories, func(m *assistantv1.AssistantMemory) bool { return slices.Contains(r.GetIds(), m.GetId()) })
	return &assistantv1.DeleteAssistantMemoriesResponse{}, nil
}

func (a *fakeAssistant) ListAssistantConversations(context.Context, *assistantv1.ListAssistantConversationsRequest) (*assistantv1.ListAssistantConversationsResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fakeAssistantPrefs()
	return assistantv1.ListAssistantConversationsResponse_builder{Conversations: slices.Clone(a.convs)}.Build(), nil
}

func (a *fakeAssistant) GetAssistantConversation(_ context.Context, r *assistantv1.GetAssistantConversationRequest) (*assistantv1.GetAssistantConversationResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fakeAssistantPrefs()
	for _, c := range a.convs {
		if c.GetId() == r.GetId() {
			return assistantv1.GetAssistantConversationResponse_builder{Conversation: c, Messages: []*assistantv1.AssistantMessage{
				assistantv1.AssistantMessage_builder{User: new(true), Text: new("Play something funny"), Time: c.GetUpdateTime()}.Build(),
				assistantv1.AssistantMessage_builder{Text: new("Playing Paddington."), Time: c.GetUpdateTime()}.Build(),
			}}.Build(), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("conversation %s", r.GetId()))
}

func (a *fakeAssistant) DeleteAssistantConversation(_ context.Context, r *assistantv1.DeleteAssistantConversationRequest) (*assistantv1.DeleteAssistantConversationResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fakeAssistantPrefs()
	a.convs = slices.DeleteFunc(a.convs, func(c *assistantv1.AssistantConversation) bool { return c.GetId() == r.GetId() })
	return &assistantv1.DeleteAssistantConversationResponse{}, nil
}

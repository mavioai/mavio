package app

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// fakeConversations is a fake server's AI conversations: one of Kids's
// with the assistant, its message after the assistant's context line, and
// those started; a message is answered in two parts, its words echoed.
type fakeConversations struct {
	systemv1connect.UnimplementedAiConversationServiceHandler

	mu       sync.Mutex
	sessions []*pluginv1.AgentSession
	messages map[string][]*pluginv1.AgentMessage
	// instructions are those of the conversations started, by their IDs.
	instructions map[string]string
}

func newFakeConversations() *fakeConversations {
	at := timestamppb.New(time.Now().Add(-2 * time.Hour))
	return &fakeConversations{
		sessions: []*pluginv1.AgentSession{pluginv1.AgentSession_builder{
			Id: new("c1"), Provider: new("anthropic"), Model: new("claude-sonnet-5-5"), Owner: new("u3"), Updated: at,
			Labels: map[string]string{"mavio.assistant": "1", "device": "Living Room"},
			Usage:  pluginv1.AiUsage_builder{InputTokens: new(int64(12345)), OutputTokens: new(int64(678)), Cost: new(0.0421)}.Build(),
		}.Build()},
		messages: map[string][]*pluginv1.AgentMessage{"c1": {
			pluginv1.AgentMessage_builder{Role: new(pluginv1.AiRole_AI_ROLE_USER), Text: new("[screen: home]\nPlay something funny"), Time: at}.Build(),
			pluginv1.AgentMessage_builder{Role: new(pluginv1.AiRole_AI_ROLE_ASSISTANT), Text: new("Playing Paddington."), Time: at}.Build(),
		}},
		instructions: map[string]string{},
	}
}

func (f *fakeConversations) ListAiConversations(context.Context, *systemv1.ListAiConversationsRequest) (*systemv1.ListAiConversationsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return systemv1.ListAiConversationsResponse_builder{Conversations: slices.Clone(f.sessions)}.Build(), nil
}

func (f *fakeConversations) GetAiConversation(_ context.Context, r *systemv1.GetAiConversationRequest) (*systemv1.GetAiConversationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sessions {
		if s.GetId() == r.GetId() {
			return systemv1.GetAiConversationResponse_builder{Conversation: s, Messages: slices.Clone(f.messages[s.GetId()])}.Build(), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("conversation %s", r.GetId()))
}

func (f *fakeConversations) CreateAiConversation(_ context.Context, r *systemv1.CreateAiConversationRequest) (*systemv1.CreateAiConversationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := fmt.Sprintf("c%d", len(f.sessions)+1)
	s := pluginv1.AgentSession_builder{
		Id: &id, Title: new(r.GetTitle()), Provider: new(r.GetProvider()), Model: new(r.GetModel()), Owner: new("u1"),
		Updated: timestamppb.Now(),
	}.Build()
	f.sessions = append([]*pluginv1.AgentSession{s}, f.sessions...)
	f.instructions[id] = r.GetInstructions()
	return systemv1.CreateAiConversationResponse_builder{Conversation: s}.Build(), nil
}

func (f *fakeConversations) SendAiMessage(_ context.Context, r *systemv1.SendAiMessageRequest, stream *connect.ServerStream[systemv1.SendAiMessageResponse]) error {
	answer := "You said: " + r.GetText()
	for _, part := range []string{"You said:", answer} {
		if err := stream.Send(systemv1.SendAiMessageResponse_builder{Text: new(part)}.Build()); err != nil {
			return err
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	now := timestamppb.Now()
	f.messages[r.GetConversationId()] = append(f.messages[r.GetConversationId()],
		pluginv1.AgentMessage_builder{Role: new(pluginv1.AiRole_AI_ROLE_USER), Text: new(r.GetText()), Time: now}.Build(),
		pluginv1.AgentMessage_builder{Role: new(pluginv1.AiRole_AI_ROLE_ASSISTANT), Text: &answer, Time: now}.Build())
	return nil
}

func (f *fakeConversations) DeleteAiConversation(_ context.Context, r *systemv1.DeleteAiConversationRequest) (*systemv1.DeleteAiConversationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = slices.DeleteFunc(f.sessions, func(s *pluginv1.AgentSession) bool { return s.GetId() == r.GetId() })
	delete(f.messages, r.GetId())
	return &systemv1.DeleteAiConversationResponse{}, nil
}

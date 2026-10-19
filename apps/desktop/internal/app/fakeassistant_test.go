package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/client-rust/capi"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1/assistantv1connect"
	intentv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/intent/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
)

// fakeAssistant is the server's assistant: a few requests it answers as
// the server would.
type fakeAssistant struct {
	assistantv1connect.UnimplementedAssistantServiceHandler
	f *fakeCore

	mu   sync.Mutex
	asks []*assistantv1.AskRequest
}

func (a *fakeAssistant) requests() []*assistantv1.AskRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]*assistantv1.AskRequest(nil), a.asks...)
}

func (a *fakeAssistant) GetAssistant(context.Context, *assistantv1.GetAssistantRequest) (*assistantv1.GetAssistantResponse, error) {
	return assistantv1.GetAssistantResponse_builder{
		Enabled: new(true), Ai: new(true), Examples: []string{"Play The Long Orbit", "Space films"},
	}.Build(), nil
}

func (a *fakeAssistant) Ask(_ context.Context, req *assistantv1.AskRequest, stream *connect.ServerStream[assistantv1.AskResponse]) error {
	a.mu.Lock()
	a.asks = append(a.asks, req)
	a.mu.Unlock()
	api := &fakeAPI{f: a.f}
	var events []*assistantv1.AskResponse
	add := func(b assistantv1.AskResponse_builder) { events = append(events, b.Build()) }
	done := func(answer string) {
		add(assistantv1.AskResponse_builder{Done: assistantv1.Done_builder{Answer: new(answer)}.Build()})
	}
	switch text := strings.ToLower(req.GetText()); {
	case text == "play the long orbit":
		add(assistantv1.AskResponse_builder{Understood: assistantv1.Understood_builder{Text: new("Play The Long Orbit"), Intent: new("play")}.Build()})
		add(assistantv1.AskResponse_builder{Tool: assistantv1.ToolActivity_builder{Id: new("t1"), Tool: new("search_library"), Summary: new("Searched the library")}.Build()})
		add(assistantv1.AskResponse_builder{Tool: assistantv1.ToolActivity_builder{Id: new("t1"), Tool: new("search_library"), Summary: new("Searched the library"), Done: new(true)}.Build()})
		add(assistantv1.AskResponse_builder{Action: assistantv1.ClientAction_builder{Id: new("a1"), Play: assistantv1.PlayAction_builder{ItemIds: []string{"orbit"}}.Build()}.Build()})
		done("Playing The Long Orbit.")
	case text == "space films":
		add(assistantv1.AskResponse_builder{Action: assistantv1.ClientAction_builder{Id: new("a2"), ShowItems: assistantv1.ShowItemsAction_builder{
			Items: []*libraryv1.Item{api.item("orbit"), api.item("lantern")}, Choose: new(true),
		}.Build()}.Build()})
		done("Which one?")
	case req.GetChoice() != "":
		add(assistantv1.AskResponse_builder{Action: assistantv1.ClientAction_builder{Id: new("a3"), Navigate: assistantv1.NavigateAction_builder{
			Target: new(assistantv1.NavigateTarget_NAVIGATE_TARGET_ITEM), Id: new(req.GetChoice()),
		}.Build()}.Build()})
		done("Here it is.")
	case text == "request dune":
		add(assistantv1.AskResponse_builder{Confirmation: assistantv1.Confirmation_builder{Id: new("c1"), Text: new("Request Dune?")}.Build()})
		add(assistantv1.AskResponse_builder{Done: assistantv1.Done_builder{Confirmation: new(true)}.Build()})
	case req.GetAnswer().GetId() == "c1" && req.GetAnswer().GetConfirmed():
		add(assistantv1.AskResponse_builder{Answer: new("Requested ")})
		add(assistantv1.AskResponse_builder{Answer: new("Dune.")})
		done("")
	default:
		add(assistantv1.AskResponse_builder{Done: assistantv1.Done_builder{Answer: new("I didn't understand."), NotUnderstood: new(true)}.Build()})
	}
	for _, e := range events {
		if err := stream.Send(e); err != nil {
			return err
		}
	}
	return nil
}

// callServerStream serves a streaming call of the fake server, giving its
// messages as parts.
func (f *fakeCore) callServerStream(ctx context.Context, c *capiv1.CallServerStream, part func(*capiv1.Message)) (*capiv1.Message, error) {
	body := binary.BigEndian.AppendUint32([]byte{0}, uint32(len(c.GetRequest())))
	body = append(body, c.GetRequest()...)
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "http://mavio.test"+c.GetProcedure(), bytes.NewReader(body))
	r.Header.Set("Content-Type", connectStream)
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, r)
	b := w.Body.Bytes()
	for len(b) >= 5 {
		flags, n := b[0], int(binary.BigEndian.Uint32(b[1:5]))
		msg := b[5 : 5+n]
		b = b[5+n:]
		if flags&envelopeEnd != 0 {
			var end struct {
				Error *struct{ Code, Message string }
			}
			_ = json.Unmarshal(msg, &end)
			if end.Error != nil {
				return nil, &capi.Error{Code: capiv1.Error_CODE_SERVER, Reason: end.Error.Message}
			}
			break
		}
		if part != nil {
			part(capiv1.Message_builder{More: true, Response: capiv1.ServerResponse_builder{Body: msg}.Build()}.Build())
		}
	}
	return capiv1.Message_builder{Done: &capiv1.Done{}}.Build(), nil
}

// runIntent is the grammar of the fake core: "pause" and "go home", and
// replies of their keys.
func (f *fakeCore) runIntent(r *capiv1.RunIntent) (*capiv1.Message, error) {
	var res proto.Message
	switch r.GetFunction() {
	case "parse":
		req := &intentv1.ParseIntentRequest{}
		if err := proto.Unmarshal(r.GetRequest(), req); err != nil {
			return nil, err
		}
		out := &intentv1.ParseIntentResponse{}
		if name, ok := map[string]string{"pause": "pause", "go home": "go_home"}[strings.ToLower(req.GetText())]; ok {
			out.SetIntents([]*intentv1.Intent{intentv1.Intent_builder{
				Name: new(name), Match: new(intentv1.IntentMatch_INTENT_MATCH_FULL), Language: new("en"),
			}.Build()})
		}
		res = out
	case "reply":
		req := &intentv1.ReplyRequest{}
		if err := proto.Unmarshal(r.GetRequest(), req); err != nil {
			return nil, err
		}
		res = intentv1.ReplyResponse_builder{Text: new(map[string]string{"paused": "Paused.", "home": "Going home."}[req.GetKey()])}.Build()
	default:
		return nil, &capi.Error{Code: capiv1.Error_CODE_INVALID, Reason: "no function " + r.GetFunction()}
	}
	b, err := proto.Marshal(res)
	if err != nil {
		return nil, err
	}
	return capiv1.Message_builder{Response: capiv1.ServerResponse_builder{Body: b}.Build()}.Build(), nil
}

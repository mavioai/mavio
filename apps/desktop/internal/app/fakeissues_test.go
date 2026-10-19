package app

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
)

// fakeIssues serves a fake server's issues: an open one with orbit's
// subtitles, reported by Kids.
type fakeIssues struct {
	acquisitionv1connect.UnimplementedIssueServiceHandler

	mu     sync.Mutex
	user   string // who reports and comments
	issues []*acquisitionv1.Issue
	next   int
}

func newFakeIssues(user string) *fakeIssues {
	at := timestamppb.New(time.Now().Add(-2 * time.Hour))
	return &fakeIssues{user: user, issues: []*acquisitionv1.Issue{acquisitionv1.Issue_builder{
		Id: new("i1"), ItemId: new("orbit"), Type: new(acquisitionv1.IssueType_ISSUE_TYPE_SUBTITLES),
		Status: new(acquisitionv1.IssueStatus_ISSUE_STATUS_OPEN), CreatedBy: new("u3"), CreateTime: at, UpdateTime: at,
		Comments: []*acquisitionv1.IssueComment{acquisitionv1.IssueComment_builder{
			Id: new("c1"), UserId: new("u3"), Message: new("The subtitles run late."), CreateTime: at,
		}.Build()},
	}.Build()}}
}

func (s *fakeIssues) find(id string) (*acquisitionv1.Issue, error) {
	i := slices.IndexFunc(s.issues, func(x *acquisitionv1.Issue) bool { return x.GetId() == id })
	if i < 0 {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return s.issues[i], nil
}

func (s *fakeIssues) CreateIssue(_ context.Context, r *acquisitionv1.CreateIssueRequest) (*acquisitionv1.CreateIssueResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	now := timestamppb.Now()
	i := acquisitionv1.Issue_builder{
		Id: new(fmt.Sprint("new", s.next)), ItemId: new(r.GetItemId()), Type: new(r.GetType()), Status: new(acquisitionv1.IssueStatus_ISSUE_STATUS_OPEN),
		SeasonNumber: new(r.GetSeasonNumber()), EpisodeNumber: new(r.GetEpisodeNumber()), CreatedBy: new(s.user), CreateTime: now, UpdateTime: now,
		Comments: []*acquisitionv1.IssueComment{acquisitionv1.IssueComment_builder{
			Id: new(fmt.Sprint("c-new", s.next)), UserId: new(s.user), Message: new(r.GetMessage()), CreateTime: now,
		}.Build()},
	}.Build()
	s.issues = append(s.issues, i)
	return acquisitionv1.CreateIssueResponse_builder{Issue: proto.CloneOf(i)}.Build(), nil
}

func (s *fakeIssues) ListIssues(_ context.Context, r *acquisitionv1.ListIssuesRequest) (*acquisitionv1.ListIssuesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*acquisitionv1.Issue
	for _, i := range s.issues {
		if (r.GetItemId() == "" || i.GetItemId() == r.GetItemId()) && (len(r.GetStatuses()) == 0 || slices.Contains(r.GetStatuses(), i.GetStatus())) {
			out = append(out, proto.CloneOf(i))
		}
	}
	return acquisitionv1.ListIssuesResponse_builder{Issues: out, Total: new(int32(len(out)))}.Build(), nil
}

func (s *fakeIssues) GetIssue(_ context.Context, r *acquisitionv1.GetIssueRequest) (*acquisitionv1.GetIssueResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.find(r.GetId())
	if err != nil {
		return nil, err
	}
	return acquisitionv1.GetIssueResponse_builder{Issue: proto.CloneOf(i)}.Build(), nil
}

func (s *fakeIssues) AddComment(_ context.Context, r *acquisitionv1.AddCommentRequest) (*acquisitionv1.AddCommentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.find(r.GetId())
	if err != nil {
		return nil, err
	}
	s.next++
	i.SetComments(append(i.GetComments(), acquisitionv1.IssueComment_builder{
		Id: new(fmt.Sprint("c-added", s.next)), UserId: new(s.user), Message: new(r.GetMessage()), CreateTime: timestamppb.Now(),
	}.Build()))
	return acquisitionv1.AddCommentResponse_builder{Issue: proto.CloneOf(i)}.Build(), nil
}

func (s *fakeIssues) setStatus(id string, status acquisitionv1.IssueStatus) (*acquisitionv1.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.find(id)
	if err != nil {
		return nil, err
	}
	i.SetStatus(status)
	return proto.CloneOf(i), nil
}

func (s *fakeIssues) ResolveIssue(_ context.Context, r *acquisitionv1.ResolveIssueRequest) (*acquisitionv1.ResolveIssueResponse, error) {
	i, err := s.setStatus(r.GetId(), acquisitionv1.IssueStatus_ISSUE_STATUS_RESOLVED)
	return acquisitionv1.ResolveIssueResponse_builder{Issue: i}.Build(), err
}

func (s *fakeIssues) ReopenIssue(_ context.Context, r *acquisitionv1.ReopenIssueRequest) (*acquisitionv1.ReopenIssueResponse, error) {
	i, err := s.setStatus(r.GetId(), acquisitionv1.IssueStatus_ISSUE_STATUS_OPEN)
	return acquisitionv1.ReopenIssueResponse_builder{Issue: i}.Build(), err
}

func (s *fakeIssues) DeleteIssue(_ context.Context, r *acquisitionv1.DeleteIssueRequest) (*acquisitionv1.DeleteIssueResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issues = slices.DeleteFunc(s.issues, func(x *acquisitionv1.Issue) bool { return x.GetId() == r.GetId() })
	return &acquisitionv1.DeleteIssueResponse{}, nil
}

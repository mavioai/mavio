package requests

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// The activity types of requests and issues, as Seerr's notification
// types. Each activity's UserID is the user it concerns: the requester
// or the issue's reporter, or the acting user for those that go to
// administrators.
const (
	// TypePending is a request waiting for approval, for administrators.
	TypePending = "request.pending"
	// TypeApproved is a request approved, for its user.
	TypeApproved = "request.approved"
	// TypeAutoApproved is a request approved at once.
	TypeAutoApproved = "request.auto_approved"
	// TypeDeclined is a request declined, for its user.
	TypeDeclined = "request.declined"
	// TypeFailed is an approved request whose movie or seasons could not
	// be made wanted.
	TypeFailed = "request.failed"
	// TypeAvailable is a request's movie or seasons available, for its
	// user.
	TypeAvailable = "request.available"
	// TypeIssueCreated is an issue reported, for administrators.
	TypeIssueCreated = "issue.created"
	// TypeIssueCommented is a comment on an issue: for its reporter when
	// another user comments, else for administrators.
	TypeIssueCommented = "issue.commented"
	// TypeIssueResolved and TypeIssueReopened are an issue resolved or
	// reopened, for its reporter.
	TypeIssueResolved = "issue.resolved"
	TypeIssueReopened = "issue.reopened"
)

// The attributes of request and issue activities.
const (
	AttrRequest   = "request_id"
	AttrIssue     = "issue_id"
	AttrComment   = "comment_id"
	AttrKind      = "media_kind"
	AttrTmdb      = "tmdb_id"
	AttrSeasons   = "seasons"
	AttrActor     = "actor_id"
	AttrIssueType = "issue_type"
)

// displayName names a request's title.
func displayName(r *core.Request) string {
	name := r.Title
	if name == "" {
		name = fmt.Sprintf("TMDB %d", r.TmdbID)
	}
	if r.Year > 0 {
		name += fmt.Sprintf(" (%d)", r.Year)
	}
	if len(r.Seasons) > 0 {
		nums := make([]string, len(r.Seasons))
		for i, n := range r.SeasonNumbers() {
			nums[i] = strconv.Itoa(n)
		}
		name += ", seasons " + strings.Join(nums, ", ")
	}
	return name
}

// notify records a request's activity for a user.
func (s *Service) notify(ctx context.Context, typ string, r *core.Request, userID, actor core.ID) {
	name := displayName(r)
	sev := core.SeverityInfo
	var title, msg string
	switch typ {
	case TypePending:
		title = name + " was requested"
	case TypeApproved:
		title = "Request of " + name + " approved"
	case TypeAutoApproved:
		title = name + " was requested and approved"
	case TypeDeclined:
		title = "Request of " + name + " declined"
	case TypeFailed:
		title, msg, sev = "Request of "+name+" failed", r.Message, core.SeverityWarning
	case TypeAvailable:
		title = name + " is available"
	}
	attrs := map[string]string{AttrRequest: r.ID.String(), AttrKind: string(r.Kind), AttrTmdb: strconv.Itoa(r.TmdbID)}
	if len(r.Seasons) > 0 {
		nums := make([]string, len(r.Seasons))
		for i, n := range r.SeasonNumbers() {
			nums[i] = strconv.Itoa(n)
		}
		attrs[AttrSeasons] = strings.Join(nums, ",")
	}
	if !actor.IsZero() {
		attrs[AttrActor] = actor.String()
	}
	s.record(ctx, core.Activity{
		Type: typ, Severity: sev, Title: title, Message: msg, UserID: userID, ItemID: r.ItemID, Attributes: attrs,
	})
}

// notifyIssue records an issue's activity for a user.
func (s *Service) notifyIssue(ctx context.Context, typ string, i *core.Issue, userID, actor core.ID, comment *core.IssueComment, name string) {
	var title, msg string
	switch typ {
	case TypeIssueCreated:
		title = fmt.Sprintf("New %s issue with %s", i.Type, name)
	case TypeIssueCommented:
		title = "New comment on the issue with " + name
	case TypeIssueResolved:
		title = "Issue with " + name + " resolved"
	case TypeIssueReopened:
		title = "Issue with " + name + " reopened"
	}
	attrs := map[string]string{AttrIssue: i.ID.String(), AttrIssueType: string(i.Type), AttrActor: actor.String()}
	if comment != nil {
		attrs[AttrComment] = comment.ID.String()
		msg = comment.Message
	}
	s.record(ctx, core.Activity{Type: typ, Title: title, Message: msg, UserID: userID, ItemID: i.ItemID, Attributes: attrs})
}

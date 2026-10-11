package requests

import (
	"context"
	"fmt"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// NewIssue is an issue to report.
type NewIssue struct {
	ItemID                      core.ID
	Type                        core.IssueType
	SeasonNumber, EpisodeNumber int
	// Message says what the problem is; it is the first comment.
	Message string
}

// CreateIssue reports a problem with an item the caller can access, as
// Seerr's POST /issue (server/routes/issue.ts): users who may report
// issues, and administrators.
func (s *Service) CreateIssue(ctx context.Context, caller core.User, n NewIssue) (core.Issue, error) {
	if !caller.Admin && !caller.Policy.Requests.ReportIssues {
		return core.Issue{}, fmt.Errorf("%w: %s may not report issues", ErrPermission, caller.Name)
	}
	if strings.TrimSpace(n.Message) == "" {
		return core.Issue{}, fmt.Errorf("%w: an issue needs a message", core.ErrInvalid)
	}
	it, err := s.Store.Items().Get(ctx, n.ItemID)
	if err != nil {
		return core.Issue{}, err
	}
	if !caller.CanAccess(&it) {
		return core.Issue{}, fmt.Errorf("item %s: %w", n.ItemID, core.ErrNotFound)
	}
	now := s.now()
	i := core.Issue{
		ID: core.NewID(), ItemID: it.ID, Type: n.Type, Status: core.IssueOpen, SeasonNumber: n.SeasonNumber, EpisodeNumber: n.EpisodeNumber,
		CreatedBy: caller.ID, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Store.Issues().Put(ctx, &i); err != nil {
		return core.Issue{}, err
	}
	c := core.IssueComment{ID: core.NewID(), IssueID: i.ID, UserID: caller.ID, Message: n.Message, CreatedAt: now, UpdatedAt: now}
	if err := s.Store.Issues().AddComment(ctx, &c); err != nil {
		return core.Issue{}, err
	}
	i.Comments = []core.IssueComment{c}
	s.notifyIssue(ctx, TypeIssueCreated, &i, caller.ID, caller.ID, &c, it.Name)
	return i, nil
}

// GetIssue returns an issue of the caller's, or anyone's for
// administrators.
func (s *Service) GetIssue(ctx context.Context, caller core.User, id core.ID) (core.Issue, error) {
	i, err := s.Store.Issues().Get(ctx, id)
	if err != nil {
		return core.Issue{}, err
	}
	if !caller.Admin && i.CreatedBy != caller.ID {
		return core.Issue{}, fmt.Errorf("%w: issue of another user", ErrPermission)
	}
	return i, nil
}

// ListIssues lists the caller's issues, or anyone's for administrators,
// as Seerr's GET /issue.
func (s *Service) ListIssues(ctx context.Context, caller core.User, q core.IssueQuery) (core.Page[core.Issue], error) {
	if !caller.Admin {
		if !q.CreatedBy.IsZero() && q.CreatedBy != caller.ID {
			return core.Page[core.Issue]{}, fmt.Errorf("%w: issues of another user", ErrPermission)
		}
		q.CreatedBy = caller.ID
	}
	return s.Store.Issues().List(ctx, q)
}

// itemName names an issue's item.
func (s *Service) itemName(ctx context.Context, i *core.Issue) string {
	it, err := s.Store.Items().Get(ctx, i.ItemID)
	if err != nil {
		return "item " + i.ItemID.String()
	}
	return it.Name
}

// Comment comments on an issue, as Seerr's POST /issue/:id/comment: its
// reporter or an administrator. A comment of another user than the
// reporter is for the reporter.
func (s *Service) Comment(ctx context.Context, caller core.User, id core.ID, message string) (core.Issue, error) {
	if strings.TrimSpace(message) == "" {
		return core.Issue{}, fmt.Errorf("%w: a comment needs a message", core.ErrInvalid)
	}
	i, err := s.GetIssue(ctx, caller, id)
	if err != nil {
		return core.Issue{}, err
	}
	now := s.now()
	c := core.IssueComment{ID: core.NewID(), IssueID: i.ID, UserID: caller.ID, Message: message, CreatedAt: now, UpdatedAt: now}
	if err := s.Store.Issues().AddComment(ctx, &c); err != nil {
		return core.Issue{}, err
	}
	i.UpdatedAt = now
	if err := s.Store.Issues().Put(ctx, &i); err != nil {
		return core.Issue{}, err
	}
	i.Comments = append(i.Comments, c)
	s.notifyIssue(ctx, TypeIssueCommented, &i, i.CreatedBy, caller.ID, &c, s.itemName(ctx, &i))
	return i, nil
}

// Resolve resolves an issue, as Seerr's POST /issue/:id/resolved: its
// reporter or an administrator.
func (s *Service) Resolve(ctx context.Context, caller core.User, id core.ID) (core.Issue, error) {
	return s.setIssueStatus(ctx, caller, id, core.IssueResolved, TypeIssueResolved)
}

// Reopen reopens an issue, as Seerr's POST /issue/:id/open: its reporter
// or an administrator.
func (s *Service) Reopen(ctx context.Context, caller core.User, id core.ID) (core.Issue, error) {
	return s.setIssueStatus(ctx, caller, id, core.IssueOpen, TypeIssueReopened)
}

func (s *Service) setIssueStatus(ctx context.Context, caller core.User, id core.ID, status core.IssueStatus, typ string) (core.Issue, error) {
	i, err := s.GetIssue(ctx, caller, id)
	if err != nil {
		return core.Issue{}, err
	}
	if i.Status == status {
		return i, nil
	}
	i.Status, i.ModifiedBy, i.UpdatedAt = status, caller.ID, s.now()
	if err := s.Store.Issues().Put(ctx, &i); err != nil {
		return core.Issue{}, err
	}
	s.notifyIssue(ctx, typ, &i, i.CreatedBy, caller.ID, nil, s.itemName(ctx, &i))
	return i, nil
}

// DeleteIssue deletes an issue, as Seerr's DELETE /issue/:id:
// administrators any, its reporter while nobody else commented on it.
func (s *Service) DeleteIssue(ctx context.Context, caller core.User, id core.ID) error {
	i, err := s.GetIssue(ctx, caller, id)
	if err != nil {
		return err
	}
	if !caller.Admin && len(i.Comments) > 1 {
		return fmt.Errorf("%w: the issue has comments", ErrPermission)
	}
	return s.Store.Issues().Delete(ctx, id)
}

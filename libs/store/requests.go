package store

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/issue"
	"github.com/mavioai/mavio/libs/store/internal/ent/issuecomment"
	"github.com/mavioai/mavio/libs/store/internal/ent/predicate"
	"github.com/mavioai/mavio/libs/store/internal/ent/request"
)

type requests struct{ s *Store }

// Requests returns users' requests.
func (s *Store) Requests() core.RequestRepository { return requests{s} }

func (r requests) Get(ctx context.Context, id core.ID) (core.Request, error) {
	row, err := r.s.read.Request.Get(ctx, id)
	if err != nil {
		return core.Request{}, mapErr(err, "get request")
	}
	return toRequest(row), nil
}

func (r requests) List(ctx context.Context, q core.RequestQuery) (core.Page[core.Request], error) {
	if err := q.Validate(); err != nil {
		return core.Page[core.Request]{}, err
	}
	limit, err := pageSize(q.Limit, q.Offset)
	if err != nil {
		return core.Page[core.Request]{}, err
	}
	var where []predicate.Request
	if !q.RequestedBy.IsZero() {
		where = append(where, request.RequestedBy(q.RequestedBy))
	}
	if len(q.Statuses) > 0 {
		statuses := make([]string, len(q.Statuses))
		for i, s := range q.Statuses {
			statuses[i] = string(s)
		}
		where = append(where, request.StatusIn(statuses...))
	}
	if q.Kind != "" {
		where = append(where, request.Kind(string(q.Kind)))
	}
	if q.TmdbID != 0 {
		where = append(where, request.TmdbID(q.TmdbID))
	}
	if !q.ItemID.IsZero() {
		where = append(where, request.ItemID(q.ItemID))
	}
	query := r.s.read.Request.Query().Where(where...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.Request]{}, mapErr(err, "count requests")
	}
	by := request.FieldCreatedAt
	if q.SortByModified {
		by = request.FieldUpdatedAt
	}
	order := ent.Desc
	if q.Ascending {
		order = ent.Asc
	}
	rows, err := query.Order(order(by), order(request.FieldID)).Limit(limit).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.Request]{}, mapErr(err, "list requests")
	}
	out := core.Page[core.Request]{Total: total, Items: make([]core.Request, len(rows))}
	for i, row := range rows {
		out.Items[i] = toRequest(row)
	}
	return out, nil
}

// Put stores the request; zero times are now.
func (r requests) Put(ctx context.Context, req *core.Request) error {
	if req.ID.IsZero() {
		req.ID = core.NewID()
	}
	if err := req.Validate(); err != nil {
		return err
	}
	req.CreatedAt = orNow(req.CreatedAt).UTC()
	req.UpdatedAt = orNow(req.UpdatedAt).UTC()
	err := r.s.write.Request.Create().
		SetID(req.ID).
		SetKind(string(req.Kind)).
		SetTmdbID(req.TmdbID).
		SetTvdbID(req.TvdbID).
		SetTitle(req.Title).
		SetYear(req.Year).
		SetNillableItemID(nonZeroID(req.ItemID)).
		SetSeasons(req.Seasons).
		SetSeasonCount(len(req.Seasons)).
		SetStatus(string(req.Status)).
		SetRequestedBy(req.RequestedBy).
		SetNillableModifiedBy(nonZeroID(req.ModifiedBy)).
		SetIgnoreQuota(req.IgnoreQuota).
		SetMessage(req.Message).
		SetCreatedAt(req.CreatedAt).
		SetUpdatedAt(req.UpdatedAt).
		OnConflictColumns(request.FieldID).
		UpdateNewValues().
		Update(func(u *ent.RequestUpsert) {
			if req.ItemID.IsZero() {
				u.ClearItemID()
			}
			if req.ModifiedBy.IsZero() {
				u.ClearModifiedBy()
			}
		}).
		Exec(ctx)
	return mapErr(err, "put request")
}

func (r requests) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.Request.DeleteOneID(id).Exec(ctx), "delete request")
}

func (r requests) QuotaUsed(ctx context.Context, userID core.ID, kind core.ItemKind, since time.Time) (int, error) {
	where := []predicate.Request{
		request.RequestedBy(userID),
		request.Kind(string(kind)),
		request.StatusNEQ(string(core.RequestDeclined)),
		request.IgnoreQuota(false),
	}
	if !since.IsZero() {
		where = append(where, request.CreatedAtGT(since.UTC()))
	}
	query := r.s.read.Request.Query().Where(where...)
	if kind != core.KindSeries {
		n, err := query.Count(ctx)
		return n, mapErr(err, "count requests")
	}
	counts, err := query.Select(request.FieldSeasonCount).Ints(ctx)
	if err != nil {
		return 0, mapErr(err, "count requested seasons")
	}
	n := 0
	for _, c := range counts {
		n += c
	}
	return n, nil
}

func toRequest(r *ent.Request) core.Request {
	out := core.Request{
		ID: r.ID, Kind: core.ItemKind(r.Kind), TmdbID: r.TmdbID, TvdbID: r.TvdbID, Title: r.Title, Year: r.Year,
		Seasons: r.Seasons, Status: core.RequestStatus(r.Status), RequestedBy: r.RequestedBy,
		IgnoreQuota: r.IgnoreQuota, Message: r.Message, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if r.ItemID != nil {
		out.ItemID = *r.ItemID
	}
	if r.ModifiedBy != nil {
		out.ModifiedBy = *r.ModifiedBy
	}
	return out
}

type issues struct{ s *Store }

// Issues returns the issues users report.
func (s *Store) Issues() core.IssueRepository { return issues{s} }

func withComments(q *ent.IssueCommentQuery) {
	q.Order(ent.Asc(issuecomment.FieldCreatedAt), ent.Asc(issuecomment.FieldID))
}

func (r issues) Get(ctx context.Context, id core.ID) (core.Issue, error) {
	row, err := r.s.read.Issue.Query().Where(issue.ID(id)).WithComments(withComments).Only(ctx)
	if err != nil {
		return core.Issue{}, mapErr(err, "get issue")
	}
	return toIssue(row), nil
}

func (r issues) List(ctx context.Context, q core.IssueQuery) (core.Page[core.Issue], error) {
	if err := q.Validate(); err != nil {
		return core.Page[core.Issue]{}, err
	}
	limit, err := pageSize(q.Limit, q.Offset)
	if err != nil {
		return core.Page[core.Issue]{}, err
	}
	var where []predicate.Issue
	if !q.CreatedBy.IsZero() {
		where = append(where, issue.CreatedBy(q.CreatedBy))
	}
	if !q.ItemID.IsZero() {
		where = append(where, issue.ItemID(q.ItemID))
	}
	if len(q.Statuses) > 0 {
		statuses := make([]string, len(q.Statuses))
		for i, s := range q.Statuses {
			statuses[i] = string(s)
		}
		where = append(where, issue.StatusIn(statuses...))
	}
	query := r.s.read.Issue.Query().Where(where...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.Issue]{}, mapErr(err, "count issues")
	}
	by := issue.FieldCreatedAt
	if q.SortByModified {
		by = issue.FieldUpdatedAt
	}
	rows, err := query.WithComments(withComments).Order(ent.Desc(by), ent.Desc(issue.FieldID)).
		Limit(limit).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.Issue]{}, mapErr(err, "list issues")
	}
	out := core.Page[core.Issue]{Total: total, Items: make([]core.Issue, len(rows))}
	for i, row := range rows {
		out.Items[i] = toIssue(row)
	}
	return out, nil
}

// Put stores the issue, leaving its comments; zero times are now.
func (r issues) Put(ctx context.Context, i *core.Issue) error {
	if i.ID.IsZero() {
		i.ID = core.NewID()
	}
	if err := i.Validate(); err != nil {
		return err
	}
	i.CreatedAt = orNow(i.CreatedAt).UTC()
	i.UpdatedAt = orNow(i.UpdatedAt).UTC()
	err := r.s.write.Issue.Create().
		SetID(i.ID).
		SetItemID(i.ItemID).
		SetType(string(i.Type)).
		SetStatus(string(i.Status)).
		SetSeasonNumber(i.SeasonNumber).
		SetEpisodeNumber(i.EpisodeNumber).
		SetCreatedBy(i.CreatedBy).
		SetNillableModifiedBy(nonZeroID(i.ModifiedBy)).
		SetCreatedAt(i.CreatedAt).
		SetUpdatedAt(i.UpdatedAt).
		OnConflictColumns(issue.FieldID).
		UpdateNewValues().
		Update(func(u *ent.IssueUpsert) {
			if i.ModifiedBy.IsZero() {
				u.ClearModifiedBy()
			}
		}).
		Exec(ctx)
	return mapErr(err, "put issue")
}

// AddComment stores a new comment; zero times are now.
func (r issues) AddComment(ctx context.Context, c *core.IssueComment) error {
	if c.ID.IsZero() {
		c.ID = core.NewID()
	}
	if err := c.Validate(); err != nil {
		return err
	}
	c.CreatedAt = orNow(c.CreatedAt).UTC()
	c.UpdatedAt = orNow(c.UpdatedAt).UTC()
	err := r.s.write.IssueComment.Create().
		SetID(c.ID).
		SetIssueID(c.IssueID).
		SetUserID(c.UserID).
		SetMessage(c.Message).
		SetCreatedAt(c.CreatedAt).
		SetUpdatedAt(c.UpdatedAt).
		Exec(ctx)
	return mapErr(err, "add issue comment")
}

func (r issues) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.Issue.DeleteOneID(id).Exec(ctx), "delete issue")
}

func toIssue(i *ent.Issue) core.Issue {
	out := core.Issue{
		ID: i.ID, ItemID: i.ItemID, Type: core.IssueType(i.Type), Status: core.IssueStatus(i.Status),
		SeasonNumber: i.SeasonNumber, EpisodeNumber: i.EpisodeNumber, CreatedBy: i.CreatedBy,
		CreatedAt: i.CreatedAt.UTC(), UpdatedAt: i.UpdatedAt.UTC(),
	}
	if i.ModifiedBy != nil {
		out.ModifiedBy = *i.ModifiedBy
	}
	for _, c := range i.Edges.Comments {
		out.Comments = append(out.Comments, core.IssueComment{
			ID: c.ID, IssueID: c.IssueID, UserID: c.UserID, Message: c.Message,
			CreatedAt: c.CreatedAt.UTC(), UpdatedAt: c.UpdatedAt.UTC(),
		})
	}
	return out
}

package rpc

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/requests"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// requestError maps the errors of the requests service to Connect codes.
func requestError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, requests.ErrPermission), errors.Is(err, requests.ErrRating):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, requests.ErrQuota):
		return connect.NewError(connect.CodeResourceExhausted, err)
	case errors.Is(err, requests.ErrState):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connectError(ctx, err)
}

// parseOptionalID parses an ID that may be empty.
func parseOptionalID(s string) (core.ID, error) {
	if s == "" {
		return core.NilID, nil
	}
	id, err := core.ParseID(s)
	if err != nil {
		return core.NilID, invalid(err)
	}
	return id, nil
}

// optionalID returns the string of an ID, nil when it is zero.
func optionalID(id core.ID) *string {
	if id.IsZero() {
		return nil
	}
	return new(id.String())
}

var requestStatuses = map[core.RequestStatus]acquisitionv1.RequestStatus{
	core.RequestPending:   acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING,
	core.RequestApproved:  acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED,
	core.RequestDeclined:  acquisitionv1.RequestStatus_REQUEST_STATUS_DECLINED,
	core.RequestFailed:    acquisitionv1.RequestStatus_REQUEST_STATUS_FAILED,
	core.RequestCompleted: acquisitionv1.RequestStatus_REQUEST_STATUS_COMPLETED,
}

func requestToProto(r *core.Request) *acquisitionv1.Request {
	seasons := make([]*acquisitionv1.SeasonRequest, len(r.Seasons))
	for i, s := range r.Seasons {
		seasons[i] = acquisitionv1.SeasonRequest_builder{
			SeasonNumber: new(int32(s.Number)), Status: new(requestStatuses[s.Status]),
		}.Build()
	}
	return acquisitionv1.Request_builder{
		Id: new(r.ID.String()), MediaKind: new(keyOf(acquisitionKinds, r.Kind)), TmdbId: new(int32(r.TmdbID)),
		TvdbId: new(int32(r.TvdbID)), Title: &r.Title, Year: new(int32(r.Year)), ItemId: optionalID(r.ItemID),
		Seasons: seasons, Status: new(requestStatuses[r.Status]), RequestedBy: new(r.RequestedBy.String()),
		ModifiedBy: optionalID(r.ModifiedBy), IgnoreQuota: &r.IgnoreQuota, Message: &r.Message,
		CreateTime: timestamp(r.CreatedAt), UpdateTime: timestamp(r.UpdatedAt),
	}.Build()
}

// RequestService implements mavio.acquisition.v1.RequestService.
type RequestService struct{ requests *requests.Service }

var _ acquisitionv1connect.RequestServiceHandler = (*RequestService)(nil)

// NewRequestService returns a RequestService backed by the requests
// service.
func NewRequestService(svc *requests.Service) *RequestService { return &RequestService{requests: svc} }

// CreateRequest requests a movie or seasons of a series.
func (s *RequestService) CreateRequest(ctx context.Context, req *acquisitionv1.CreateRequestRequest) (*acquisitionv1.CreateRequestResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := mediaKind(req.GetMediaKind())
	if err != nil {
		return nil, err
	}
	forID, err := parseOptionalID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	n := requests.NewRequest{
		Kind: kind, TmdbID: int(req.GetTmdbId()), AllSeasons: req.GetAllSeasons(), For: forID, IgnoreQuota: req.GetIgnoreQuota(),
	}
	for _, sn := range req.GetSeasons() {
		n.Seasons = append(n.Seasons, int(sn))
	}
	r, err := s.requests.Create(ctx, p.User, n)
	if err != nil {
		return nil, requestError(ctx, err)
	}
	return acquisitionv1.CreateRequestResponse_builder{Request: requestToProto(&r)}.Build(), nil
}

// ListRequests lists the caller's requests, or anyone's for
// administrators.
func (s *RequestService) ListRequests(ctx context.Context, req *acquisitionv1.ListRequestsRequest) (*acquisitionv1.ListRequestsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	q := core.RequestQuery{
		TmdbID: int(req.GetTmdbId()), SortByModified: req.GetSort() == acquisitionv1.RequestSort_REQUEST_SORT_MODIFIED,
		Ascending: req.GetAscending(), Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}
	if q.RequestedBy, err = parseOptionalID(req.GetUserId()); err != nil {
		return nil, err
	}
	if k := req.GetMediaKind(); k != acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED {
		if q.Kind, err = mediaKind(k); err != nil {
			return nil, err
		}
	}
	for _, st := range req.GetStatuses() {
		q.Statuses = append(q.Statuses, keyOf(requestStatuses, st))
	}
	page, err := s.requests.List(ctx, p.User, q)
	if err != nil {
		return nil, requestError(ctx, err)
	}
	out := make([]*acquisitionv1.Request, len(page.Items))
	for i := range page.Items {
		out[i] = requestToProto(&page.Items[i])
	}
	return acquisitionv1.ListRequestsResponse_builder{Requests: out, Total: new(int32(page.Total))}.Build(), nil
}

// requestCall runs a call on a request by its ID with the caller.
func requestCall(ctx context.Context, id string, call func(core.User, core.ID) (core.Request, error)) (*acquisitionv1.Request, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	rid, err := core.ParseID(id)
	if err != nil {
		return nil, invalid(err)
	}
	r, err := call(p.User, rid)
	if err != nil {
		return nil, requestError(ctx, err)
	}
	return requestToProto(&r), nil
}

// GetRequest returns a request of the caller's, or anyone's for
// administrators.
func (s *RequestService) GetRequest(ctx context.Context, req *acquisitionv1.GetRequestRequest) (*acquisitionv1.GetRequestResponse, error) {
	r, err := requestCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Request, error) { return s.requests.Get(ctx, u, id) })
	if err != nil {
		return nil, err
	}
	return acquisitionv1.GetRequestResponse_builder{Request: r}.Build(), nil
}

// UpdateRequest changes the seasons of a pending series request.
func (s *RequestService) UpdateRequest(ctx context.Context, req *acquisitionv1.UpdateRequestRequest) (*acquisitionv1.UpdateRequestResponse, error) {
	seasons := make([]int, len(req.GetSeasons()))
	for i, sn := range req.GetSeasons() {
		seasons[i] = int(sn)
	}
	r, err := requestCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Request, error) {
		return s.requests.Update(ctx, u, id, seasons)
	})
	if err != nil {
		return nil, err
	}
	return acquisitionv1.UpdateRequestResponse_builder{Request: r}.Build(), nil
}

// DeleteRequest deletes a request.
func (s *RequestService) DeleteRequest(ctx context.Context, req *acquisitionv1.DeleteRequestRequest) (*acquisitionv1.DeleteRequestResponse, error) {
	_, err := requestCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Request, error) {
		return core.Request{}, s.requests.Delete(ctx, u, id)
	})
	if err != nil {
		return nil, err
	}
	return &acquisitionv1.DeleteRequestResponse{}, nil
}

// ApproveRequest approves a pending request; administrators.
func (s *RequestService) ApproveRequest(ctx context.Context, req *acquisitionv1.ApproveRequestRequest) (*acquisitionv1.ApproveRequestResponse, error) {
	r, err := requestCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Request, error) { return s.requests.Approve(ctx, u, id) })
	if err != nil {
		return nil, err
	}
	return acquisitionv1.ApproveRequestResponse_builder{Request: r}.Build(), nil
}

// DeclineRequest declines a pending request; administrators.
func (s *RequestService) DeclineRequest(ctx context.Context, req *acquisitionv1.DeclineRequestRequest) (*acquisitionv1.DeclineRequestResponse, error) {
	r, err := requestCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Request, error) { return s.requests.Decline(ctx, u, id) })
	if err != nil {
		return nil, err
	}
	return acquisitionv1.DeclineRequestResponse_builder{Request: r}.Build(), nil
}

// RetryRequest approves a failed request again; administrators.
func (s *RequestService) RetryRequest(ctx context.Context, req *acquisitionv1.RetryRequestRequest) (*acquisitionv1.RetryRequestResponse, error) {
	r, err := requestCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Request, error) { return s.requests.Retry(ctx, u, id) })
	if err != nil {
		return nil, err
	}
	return acquisitionv1.RetryRequestResponse_builder{Request: r}.Build(), nil
}

// GetQuota returns the caller's quotas, or any user's for administrators.
func (s *RequestService) GetQuota(ctx context.Context, req *acquisitionv1.GetQuotaRequest) (*acquisitionv1.GetQuotaResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	u := p.User
	if req.GetUserId() != "" && req.GetUserId() != u.ID.String() {
		if !u.IsAdmin() {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("quotas of another user"))
		}
		id, err := core.ParseID(req.GetUserId())
		if err != nil {
			return nil, invalid(err)
		}
		if u, err = s.requests.Store.Users().Get(ctx, id); err != nil {
			return nil, connectError(ctx, err)
		}
	}
	out := acquisitionv1.GetQuotaResponse_builder{}
	for _, k := range []struct {
		kind core.ItemKind
		to   **acquisitionv1.Quota
	}{{core.KindMovie, &out.Movie}, {core.KindSeries, &out.Series}} {
		q, err := s.requests.Quota(ctx, u, k.kind)
		if err != nil {
			return nil, requestError(ctx, err)
		}
		*k.to = acquisitionv1.Quota_builder{
			Limit: new(int32(q.Limit)), Days: new(int32(q.Days)), Used: new(int32(q.Used)),
			Remaining: new(int32(q.Remaining)), Restricted: new(q.Restricted),
		}.Build()
	}
	return out.Build(), nil
}

func blockedTitleToProto(b *core.BlockedTitle) *acquisitionv1.BlockedTitle {
	return acquisitionv1.BlockedTitle_builder{
		MediaKind: new(keyOf(acquisitionKinds, b.Kind)), TmdbId: new(int32(b.TmdbID)), Title: new(b.Title),
		UserId: optionalID(b.UserID), CreateTime: timestamp(b.CreatedAt), Tags: b.Tags,
	}.Build()
}

var blockedTitleSources = map[core.BlockedTitleSource]acquisitionv1.BlockedTitleSource{
	core.BlockedByHand: acquisitionv1.BlockedTitleSource_BLOCKED_TITLE_SOURCE_MANUAL,
	core.BlockedByTags: acquisitionv1.BlockedTitleSource_BLOCKED_TITLE_SOURCE_TAGS,
}

// BlockCollection blocklists the movies of a collection; administrators.
func (s *RequestService) BlockCollection(ctx context.Context, req *acquisitionv1.BlockCollectionRequest) (*acquisitionv1.BlockCollectionResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	blocked, err := s.requests.BlockCollection(ctx, p.User, int(req.GetTmdbCollectionId()))
	if err != nil {
		return nil, requestError(ctx, err)
	}
	out := make([]*acquisitionv1.BlockedTitle, len(blocked))
	for i := range blocked {
		out[i] = blockedTitleToProto(&blocked[i])
	}
	return acquisitionv1.BlockCollectionResponse_builder{BlockedTitles: out}.Build(), nil
}

// UnblockCollection takes the movies of a collection off the blocklist;
// administrators.
func (s *RequestService) UnblockCollection(ctx context.Context, req *acquisitionv1.UnblockCollectionRequest) (*acquisitionv1.UnblockCollectionResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requests.UnblockCollection(ctx, p.User, int(req.GetTmdbCollectionId())); err != nil {
		return nil, requestError(ctx, err)
	}
	return &acquisitionv1.UnblockCollectionResponse{}, nil
}

// BlockTitle puts a title on the blocklist; administrators.
func (s *RequestService) BlockTitle(ctx context.Context, req *acquisitionv1.BlockTitleRequest) (*acquisitionv1.BlockTitleResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := mediaKind(req.GetMediaKind())
	if err != nil {
		return nil, err
	}
	b, err := s.requests.Block(ctx, p.User, kind, int(req.GetTmdbId()))
	if err != nil {
		return nil, requestError(ctx, err)
	}
	return acquisitionv1.BlockTitleResponse_builder{BlockedTitle: blockedTitleToProto(&b)}.Build(), nil
}

// UnblockTitle takes a title off the blocklist; administrators.
func (s *RequestService) UnblockTitle(ctx context.Context, req *acquisitionv1.UnblockTitleRequest) (*acquisitionv1.UnblockTitleResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := mediaKind(req.GetMediaKind())
	if err != nil {
		return nil, err
	}
	if err := s.requests.Unblock(ctx, p.User, kind, int(req.GetTmdbId())); err != nil {
		return nil, requestError(ctx, err)
	}
	return &acquisitionv1.UnblockTitleResponse{}, nil
}

// ListBlockedTitles lists the blocklist; administrators.
func (s *RequestService) ListBlockedTitles(ctx context.Context, req *acquisitionv1.ListBlockedTitlesRequest) (*acquisitionv1.ListBlockedTitlesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	source := keyOf(blockedTitleSources, req.GetSource())
	page, err := s.requests.ListBlocked(ctx, p.User, core.BlockedTitleQuery{
		Search: req.GetSearch(), Source: source, Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, requestError(ctx, err)
	}
	out := make([]*acquisitionv1.BlockedTitle, len(page.Items))
	for i := range page.Items {
		out[i] = blockedTitleToProto(&page.Items[i])
	}
	return acquisitionv1.ListBlockedTitlesResponse_builder{BlockedTitles: out, Total: new(int32(page.Total))}.Build(), nil
}

var issueTypes = map[core.IssueType]acquisitionv1.IssueType{
	core.IssueVideo:     acquisitionv1.IssueType_ISSUE_TYPE_VIDEO,
	core.IssueAudio:     acquisitionv1.IssueType_ISSUE_TYPE_AUDIO,
	core.IssueSubtitles: acquisitionv1.IssueType_ISSUE_TYPE_SUBTITLES,
	core.IssueOther:     acquisitionv1.IssueType_ISSUE_TYPE_OTHER,
}

var issueStatuses = map[core.IssueStatus]acquisitionv1.IssueStatus{
	core.IssueOpen:     acquisitionv1.IssueStatus_ISSUE_STATUS_OPEN,
	core.IssueResolved: acquisitionv1.IssueStatus_ISSUE_STATUS_RESOLVED,
}

func issueToProto(i *core.Issue) *acquisitionv1.Issue {
	comments := make([]*acquisitionv1.IssueComment, len(i.Comments))
	for j, c := range i.Comments {
		comments[j] = acquisitionv1.IssueComment_builder{
			Id: new(c.ID.String()), UserId: new(c.UserID.String()), Message: new(c.Message),
			CreateTime: timestamp(c.CreatedAt), UpdateTime: timestamp(c.UpdatedAt),
		}.Build()
	}
	return acquisitionv1.Issue_builder{
		Id: new(i.ID.String()), ItemId: new(i.ItemID.String()), Type: new(issueTypes[i.Type]), Status: new(issueStatuses[i.Status]),
		SeasonNumber: new(int32(i.SeasonNumber)), EpisodeNumber: new(int32(i.EpisodeNumber)),
		CreatedBy: new(i.CreatedBy.String()), ModifiedBy: optionalID(i.ModifiedBy), Comments: comments,
		CreateTime: timestamp(i.CreatedAt), UpdateTime: timestamp(i.UpdatedAt),
	}.Build()
}

// IssueService implements mavio.acquisition.v1.IssueService.
type IssueService struct{ requests *requests.Service }

var _ acquisitionv1connect.IssueServiceHandler = (*IssueService)(nil)

// NewIssueService returns an IssueService backed by the requests
// service.
func NewIssueService(svc *requests.Service) *IssueService { return &IssueService{requests: svc} }

// issueCall runs a call on an issue by its ID with the caller.
func issueCall(ctx context.Context, id string, call func(core.User, core.ID) (core.Issue, error)) (*acquisitionv1.Issue, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	iid, err := core.ParseID(id)
	if err != nil {
		return nil, invalid(err)
	}
	i, err := call(p.User, iid)
	if err != nil {
		return nil, requestError(ctx, err)
	}
	return issueToProto(&i), nil
}

// CreateIssue reports a problem with an item.
func (s *IssueService) CreateIssue(ctx context.Context, req *acquisitionv1.CreateIssueRequest) (*acquisitionv1.CreateIssueResponse, error) {
	n := requests.NewIssue{
		Type: keyOf(issueTypes, req.GetType()), SeasonNumber: int(req.GetSeasonNumber()),
		EpisodeNumber: int(req.GetEpisodeNumber()), Message: req.GetMessage(),
	}
	i, err := issueCall(ctx, req.GetItemId(), func(u core.User, id core.ID) (core.Issue, error) {
		n.ItemID = id
		return s.requests.CreateIssue(ctx, u, n)
	})
	if err != nil {
		return nil, err
	}
	return acquisitionv1.CreateIssueResponse_builder{Issue: i}.Build(), nil
}

// ListIssues lists the caller's issues, or anyone's for administrators.
func (s *IssueService) ListIssues(ctx context.Context, req *acquisitionv1.ListIssuesRequest) (*acquisitionv1.ListIssuesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	q := core.IssueQuery{
		SortByModified: req.GetSort() == acquisitionv1.IssueSort_ISSUE_SORT_MODIFIED,
		Limit:          int(req.GetLimit()), Offset: int(req.GetOffset()),
	}
	if q.CreatedBy, err = parseOptionalID(req.GetUserId()); err != nil {
		return nil, err
	}
	if q.ItemID, err = parseOptionalID(req.GetItemId()); err != nil {
		return nil, err
	}
	for _, st := range req.GetStatuses() {
		q.Statuses = append(q.Statuses, keyOf(issueStatuses, st))
	}
	page, err := s.requests.ListIssues(ctx, p.User, q)
	if err != nil {
		return nil, requestError(ctx, err)
	}
	out := make([]*acquisitionv1.Issue, len(page.Items))
	for i := range page.Items {
		out[i] = issueToProto(&page.Items[i])
	}
	return acquisitionv1.ListIssuesResponse_builder{Issues: out, Total: new(int32(page.Total))}.Build(), nil
}

// GetIssue returns an issue with its comments.
func (s *IssueService) GetIssue(ctx context.Context, req *acquisitionv1.GetIssueRequest) (*acquisitionv1.GetIssueResponse, error) {
	i, err := issueCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Issue, error) { return s.requests.GetIssue(ctx, u, id) })
	if err != nil {
		return nil, err
	}
	return acquisitionv1.GetIssueResponse_builder{Issue: i}.Build(), nil
}

// AddComment comments on an issue.
func (s *IssueService) AddComment(ctx context.Context, req *acquisitionv1.AddCommentRequest) (*acquisitionv1.AddCommentResponse, error) {
	i, err := issueCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Issue, error) {
		return s.requests.Comment(ctx, u, id, req.GetMessage())
	})
	if err != nil {
		return nil, err
	}
	return acquisitionv1.AddCommentResponse_builder{Issue: i}.Build(), nil
}

// ResolveIssue resolves an issue.
func (s *IssueService) ResolveIssue(ctx context.Context, req *acquisitionv1.ResolveIssueRequest) (*acquisitionv1.ResolveIssueResponse, error) {
	i, err := issueCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Issue, error) { return s.requests.Resolve(ctx, u, id) })
	if err != nil {
		return nil, err
	}
	return acquisitionv1.ResolveIssueResponse_builder{Issue: i}.Build(), nil
}

// ReopenIssue opens a resolved issue again.
func (s *IssueService) ReopenIssue(ctx context.Context, req *acquisitionv1.ReopenIssueRequest) (*acquisitionv1.ReopenIssueResponse, error) {
	i, err := issueCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Issue, error) { return s.requests.Reopen(ctx, u, id) })
	if err != nil {
		return nil, err
	}
	return acquisitionv1.ReopenIssueResponse_builder{Issue: i}.Build(), nil
}

// DeleteIssue deletes an issue.
func (s *IssueService) DeleteIssue(ctx context.Context, req *acquisitionv1.DeleteIssueRequest) (*acquisitionv1.DeleteIssueResponse, error) {
	_, err := issueCall(ctx, req.GetId(), func(u core.User, id core.ID) (core.Issue, error) {
		return core.Issue{}, s.requests.DeleteIssue(ctx, u, id)
	})
	if err != nil {
		return nil, err
	}
	return &acquisitionv1.DeleteIssueResponse{}, nil
}

// DiscoverService implements mavio.acquisition.v1.DiscoverService.
type DiscoverService struct{ requests *requests.Service }

var _ acquisitionv1connect.DiscoverServiceHandler = (*DiscoverService)(nil)

// NewDiscoverService returns a DiscoverService backed by the requests
// service.
func NewDiscoverService(svc *requests.Service) *DiscoverService {
	return &DiscoverService{requests: svc}
}

// pluginKinds maps the media kinds of requests to those of plugins.
var pluginKinds = map[acquisitionv1.MediaKind]pluginv1.MediaKind{
	acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED: pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED,
	acquisitionv1.MediaKind_MEDIA_KIND_MOVIE:       pluginv1.MediaKind_MEDIA_KIND_MOVIE,
	acquisitionv1.MediaKind_MEDIA_KIND_SERIES:      pluginv1.MediaKind_MEDIA_KIND_SERIES,
}

// ListCatalogs lists the catalogs of the discovery providers.
func (s *DiscoverService) ListCatalogs(ctx context.Context, req *acquisitionv1.ListCatalogsRequest) (*acquisitionv1.ListCatalogsResponse, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	cs, err := s.requests.Catalogs(ctx, req.GetLanguage())
	if err != nil {
		return nil, requestError(ctx, err)
	}
	out := make([]*acquisitionv1.Catalog, len(cs))
	for i, c := range cs {
		b := acquisitionv1.Catalog_builder{
			Id: new(c.ID), Name: new(c.Catalog.GetName()), Kind: new(c.Catalog.GetKind()), Filters: c.Catalog.GetFilters(),
			Sorts: c.Catalog.GetSorts(), PluginId: new(c.PluginID), Mixed: new(false),
		}
		for _, k := range c.Catalog.GetMediaKinds() {
			switch k {
			case pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED:
				b.Mixed = new(true)
			case pluginv1.MediaKind_MEDIA_KIND_MOVIE:
				b.MediaKinds = append(b.MediaKinds, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE)
			case pluginv1.MediaKind_MEDIA_KIND_SERIES:
				b.MediaKinds = append(b.MediaKinds, acquisitionv1.MediaKind_MEDIA_KIND_SERIES)
			}
		}
		out[i] = b.Build()
	}
	return acquisitionv1.ListCatalogsResponse_builder{Catalogs: out}.Build(), nil
}

var mediaStatuses = map[requests.MediaStatus]acquisitionv1.MediaStatus{
	requests.StatusNone:        acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED,
	requests.StatusRequested:   acquisitionv1.MediaStatus_MEDIA_STATUS_REQUESTED,
	requests.StatusWanted:      acquisitionv1.MediaStatus_MEDIA_STATUS_WANTED,
	requests.StatusPartial:     acquisitionv1.MediaStatus_MEDIA_STATUS_PARTIALLY_AVAILABLE,
	requests.StatusAvailable:   acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE,
	requests.StatusBlocklisted: acquisitionv1.MediaStatus_MEDIA_STATUS_BLOCKLISTED,
	requests.StatusDeleted:     acquisitionv1.MediaStatus_MEDIA_STATUS_DELETED,
}

// Discover lists a page of a catalog, marked with what the server has of
// each title.
func (s *DiscoverService) Discover(ctx context.Context, req *acquisitionv1.DiscoverRequest) (*acquisitionv1.DiscoverResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	pr := pluginv1.DiscoverRequest_builder{
		MediaKind: new(pluginKinds[req.GetMediaKind()]), Genres: req.GetGenres(), Studios: req.GetStudios(),
		Networks: req.GetNetworks(), Keywords: req.GetKeywords(), ExcludedKeywords: req.GetExcludedKeywords(),
		WatchProviders: req.GetWatchProviders(), WatchRegion: new(req.GetWatchRegion()),
		OriginalLanguage: new(req.GetOriginalLanguage()), ReleasedFrom: new(req.GetReleasedFrom()),
		ReleasedTo: new(req.GetReleasedTo()), RuntimeMin: new(req.GetRuntimeMin()), RuntimeMax: new(req.GetRuntimeMax()),
		RatingMin: new(req.GetRatingMin()), RatingMax: new(req.GetRatingMax()), VotesMin: new(req.GetVotesMin()),
		Certifications: req.GetCertifications(), Sort: new(req.GetSort()), Ascending: new(req.GetAscending()),
		Language: new(req.GetLanguage()),
	}
	if req.GetTmdbId() > 0 {
		pr.ExternalIds = map[string]string{string(core.ProviderTMDB): strconv.Itoa(int(req.GetTmdbId()))}
	}
	if req.GetTmdbCollectionId() > 0 {
		if pr.ExternalIds == nil {
			pr.ExternalIds = map[string]string{}
		}
		pr.ExternalIds[string(core.ProviderTMDBCollection)] = strconv.Itoa(int(req.GetTmdbCollectionId()))
	}
	if req.GetPage() > 0 {
		pr.Page = new(req.GetPage())
	}
	page, err := s.requests.Discover(ctx, p.User, req.GetCatalog(), pr.Build())
	if err != nil {
		return nil, requestError(ctx, err)
	}
	return acquisitionv1.DiscoverResponse_builder{
		Titles: discoveredToProto(page.Titles), Page: new(int32(page.Page)), TotalPages: new(int32(page.TotalPages)),
		TotalResults: new(int32(page.TotalResults)), HiddenResults: new(int32(page.Hidden)),
	}.Build(), nil
}

// discoveredToProto converts discovered titles.
func discoveredToProto(titles []requests.DiscoveredTitle) []*acquisitionv1.DiscoveredTitle {
	out := make([]*acquisitionv1.DiscoveredTitle, len(titles))
	for i := range titles {
		out[i] = discoveredTitleToProto(&titles[i])
	}
	return out
}

// SearchTitles finds titles by name through a discovery provider.
func (s *DiscoverService) SearchTitles(ctx context.Context, req *acquisitionv1.SearchTitlesRequest) (*acquisitionv1.SearchTitlesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	pr := pluginv1.SearchTitlesRequest_builder{
		Query: new(req.GetQuery()), MediaKind: new(pluginKinds[req.GetMediaKind()]), Language: new(req.GetLanguage()),
	}
	if req.GetYear() > 0 {
		pr.Year = new(req.GetYear())
	}
	if req.GetPage() > 0 {
		pr.Page = new(req.GetPage())
	}
	page, err := s.requests.Search(ctx, p.User, req.GetPluginId(), pr.Build())
	if err != nil {
		return nil, requestError(ctx, err)
	}
	return acquisitionv1.SearchTitlesResponse_builder{
		Titles: discoveredToProto(page.Titles), Page: new(int32(page.Page)), TotalPages: new(int32(page.TotalPages)),
		TotalResults: new(int32(page.TotalResults)), HiddenResults: new(int32(page.Hidden)), PluginId: new(page.PluginID),
	}.Build(), nil
}

func discoveredTitleToProto(d *requests.DiscoveredTitle) *acquisitionv1.DiscoveredTitle {
	t := d.Title
	b := acquisitionv1.DiscoveredTitle_builder{
		MediaKind: new(keyOf(acquisitionKinds, d.Kind)), Name: new(t.GetName()), OriginalTitle: new(t.GetOriginalTitle()),
		OriginalLanguage: new(t.GetOriginalLanguage()), Overview: new(t.GetOverview()),
		PremiereDate: new(t.GetPremiereDate()), Year: new(t.GetYear()), ExternalIds: t.GetExternalIds(),
		Genres: t.GetGenres(), PosterUrl: new(t.GetPosterUrl()), BackdropUrl: new(t.GetBackdropUrl()),
		CommunityRating: new(t.GetCommunityRating()), VoteCount: new(t.GetVoteCount()), Popularity: new(t.GetPopularity()),
		Status: new(mediaStatuses[d.Status]), ItemId: optionalID(d.ItemID), RequestId: optionalID(d.RequestID),
		Requestable: new(d.Requestable),
	}
	if d.RequestStatus != "" {
		b.RequestStatus = new(requestStatuses[d.RequestStatus])
	}
	return b.Build()
}

// isoDate formats a date as an ISO 8601 calendar date, empty when nil.
func isoDate(t *time.Time) *string {
	if t == nil {
		return new("")
	}
	return new(t.Format(time.DateOnly))
}

// GetTitle returns a title's page with the status of each season.
func (s *DiscoverService) GetTitle(ctx context.Context, req *acquisitionv1.GetTitleRequest) (*acquisitionv1.GetTitleResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := mediaKind(req.GetMediaKind())
	if err != nil {
		return nil, err
	}
	d, err := s.requests.GetTitle(ctx, p.User, kind, int(req.GetTmdbId()), req.GetLanguage())
	if err != nil {
		return nil, requestError(ctx, err)
	}
	ids := stringIDs(d.ExternalIDs)
	reqs := make([]*acquisitionv1.Request, len(d.Requests))
	for i := range d.Requests {
		reqs[i] = requestToProto(&d.Requests[i])
	}
	seasons := make([]*acquisitionv1.SeasonStatus, len(d.Seasons))
	for i, ss := range d.Seasons {
		b := acquisitionv1.SeasonStatus_builder{
			SeasonNumber: new(int32(ss.Number)), EpisodeCount: new(int32(ss.Episodes)), PremiereDate: isoDate(ss.PremiereDate),
			Status: new(mediaStatuses[ss.Status]), RequestId: optionalID(ss.RequestID), Requestable: new(ss.Requestable),
		}
		if ss.RequestStatus != "" {
			b.RequestStatus = new(requestStatuses[ss.RequestStatus])
		}
		seasons[i] = b.Build()
	}
	credits := make([]*acquisitionv1.TitleCredit, len(d.People))
	for i, pc := range d.People {
		b := acquisitionv1.TitleCredit_builder{
			Name: new(pc.Name), Kind: new(creditKinds[pc.Kind]), Role: new(pc.Role), ImageUrl: new(pc.ImageURL), ExternalIds: stringIDs(pc.ExternalIDs),
		}
		if pc.Order != nil {
			b.Order = new(int32(*pc.Order))
		}
		credits[i] = b.Build()
	}
	watch := make([]*pluginv1.WatchProvider, len(d.WatchProviders))
	for i, w := range d.WatchProviders {
		watch[i] = pluginv1.WatchProvider_builder{Name: new(w.Name), LogoUrl: new(w.LogoURL)}.Build()
	}
	t := acquisitionv1.Title_builder{
		MediaKind: new(keyOf(acquisitionKinds, d.Kind)), TmdbId: new(int32(d.TmdbID)), TvdbId: new(int32(d.TvdbID)),
		Name: new(d.Name), OriginalTitle: new(d.OriginalTitle), Overview: new(d.Overview), PremiereDate: isoDate(d.PremiereDate),
		Year: new(int32(d.Year)), OfficialRating: new(d.Rating), RatingCountry: new(d.Country), Genres: d.Genres,
		CommunityRating: new(d.CommunityRating), ExternalIds: ids, PosterUrl: new(d.PosterURL), BackdropUrl: new(d.BackdropURL),
		Anime: new(d.Anime()), Status: new(mediaStatuses[d.Status]), ItemId: optionalID(d.ItemID), Requests: reqs,
		Requestable: new(d.Requestable), Seasons: seasons,

		OriginalLanguage: new(d.OriginalLanguage), Studios: d.Studios, ProductionLocations: d.ProductionLocations,
		RuntimeMinutes: new(int32(d.Runtime / time.Minute)), Budget: new(d.Budget), Revenue: new(d.Revenue),
		DigitalReleaseDate: isoDate(d.DigitalReleaseDate), PhysicalReleaseDate: isoDate(d.PhysicalReleaseDate),
		EndDate: isoDate(d.EndDate), NextAirDate: isoDate(d.NextAirDate), WatchProviders: watch,
		Tagline: new(d.Tagline), Keywords: d.Tags, Credits: credits, TrailerUrls: d.Trailers, CollectionName: new(d.CollectionName),
		ProductionStatus: new(productionStatuses[d.ProductionStatus]), TheatricalReleaseDate: isoDate(d.TheatricalReleaseDate),
		ProductionCountries: d.ProductionCountries, GenreRefs: refsToProto(d.GenreRefs), KeywordRefs: refsToProto(d.KeywordRefs),
		StudioRefs: refsToProto(d.StudioRefs), VoteCount: new(int32(d.VoteCount)),
	}.Build()
	return acquisitionv1.GetTitleResponse_builder{Title: t}.Build(), nil
}

// GetSeason lists the episodes of a season of a series.
func (s *DiscoverService) GetSeason(ctx context.Context, req *acquisitionv1.GetSeasonRequest) (*acquisitionv1.GetSeasonResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	eps, err := s.requests.GetSeason(ctx, p.User, int(req.GetTmdbId()), int(req.GetSeasonNumber()), req.GetLanguage())
	if err != nil {
		return nil, requestError(ctx, err)
	}
	out := make([]*acquisitionv1.SeasonEpisode, len(eps))
	for i, e := range eps {
		out[i] = acquisitionv1.SeasonEpisode_builder{
			EpisodeNumber: new(int32(e.Number)), Name: new(e.Name), Overview: new(e.Overview), AirDate: isoDate(e.AirDate), StillUrl: new(e.StillURL),
		}.Build()
	}
	return acquisitionv1.GetSeasonResponse_builder{Episodes: out}.Build(), nil
}

// GetPerson tells of a person with the titles they are credited on.
func (s *DiscoverService) GetPerson(ctx context.Context, req *acquisitionv1.GetPersonRequest) (*acquisitionv1.GetPersonResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.requests.Person(ctx, p.User, int(req.GetTmdbId()), req.GetLanguage())
	if err != nil {
		return nil, requestError(ctx, err)
	}
	titles := func(ts []requests.PersonTitle) []*acquisitionv1.PersonTitle {
		out := make([]*acquisitionv1.PersonTitle, len(ts))
		for i := range ts {
			out[i] = acquisitionv1.PersonTitle_builder{Title: discoveredTitleToProto(&ts[i].DiscoveredTitle), Role: new(ts[i].Role)}.Build()
		}
		return out
	}
	pp := d.Person
	return acquisitionv1.GetPersonResponse_builder{Person: acquisitionv1.Person_builder{
		Name: new(pp.GetName()), Biography: new(pp.GetBiography()), Birthday: new(pp.GetBirthday()), Deathday: new(pp.GetDeathday()),
		PlaceOfBirth: new(pp.GetPlaceOfBirth()), AlsoKnownAs: pp.GetAlsoKnownAs(), KnownForDepartment: new(pp.GetKnownForDepartment()),
		ImageUrl: new(pp.GetImageUrl()), ExternalIds: pp.GetExternalIds(), Cast: titles(d.Cast), Crew: titles(d.Crew),
		HiddenResults: new(int32(d.Hidden)),
	}.Build()}.Build(), nil
}

// GetCollection tells of a collection of movies with its parts.
func (s *DiscoverService) GetCollection(ctx context.Context, req *acquisitionv1.GetCollectionRequest) (*acquisitionv1.GetCollectionResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.requests.Collection(ctx, p.User, int(req.GetTmdbCollectionId()), req.GetLanguage())
	if err != nil {
		return nil, requestError(ctx, err)
	}
	c := d.Collection
	return acquisitionv1.GetCollectionResponse_builder{Collection: acquisitionv1.Collection_builder{
		Name: new(c.GetName()), Overview: new(c.GetOverview()), PosterUrl: new(c.GetPosterUrl()), BackdropUrl: new(c.GetBackdropUrl()),
		ExternalIds: c.GetExternalIds(), Parts: discoveredToProto(d.Parts), HiddenResults: new(int32(d.Hidden)),
	}.Build()}.Build(), nil
}

// productionStatuses maps production statuses to the contract's.
var productionStatuses = map[metadata.ProductionStatus]pluginv1.ProductionStatus{
	metadata.ProductionRumored:        pluginv1.ProductionStatus_PRODUCTION_STATUS_RUMORED,
	metadata.ProductionPlanned:        pluginv1.ProductionStatus_PRODUCTION_STATUS_PLANNED,
	metadata.ProductionInProduction:   pluginv1.ProductionStatus_PRODUCTION_STATUS_IN_PRODUCTION,
	metadata.ProductionPostProduction: pluginv1.ProductionStatus_PRODUCTION_STATUS_POST_PRODUCTION,
	metadata.ProductionReleased:       pluginv1.ProductionStatus_PRODUCTION_STATUS_RELEASED,
	metadata.ProductionCanceled:       pluginv1.ProductionStatus_PRODUCTION_STATUS_CANCELED,
	metadata.ProductionPilot:          pluginv1.ProductionStatus_PRODUCTION_STATUS_PILOT,
	metadata.ProductionReturning:      pluginv1.ProductionStatus_PRODUCTION_STATUS_RETURNING,
	metadata.ProductionEnded:          pluginv1.ProductionStatus_PRODUCTION_STATUS_ENDED,
}

func refsToProto(rs []metadata.Ref) []*pluginv1.Reference {
	out := make([]*pluginv1.Reference, len(rs))
	for i, r := range rs {
		out[i] = pluginv1.Reference_builder{Id: new(r.ID), Name: new(r.Name)}.Build()
	}
	return out
}

// ListFilterValues lists the values a filter of a catalog takes.
func (s *DiscoverService) ListFilterValues(ctx context.Context, req *acquisitionv1.ListFilterValuesRequest) (*acquisitionv1.ListFilterValuesResponse, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	pr := pluginv1.ListFilterValuesRequest_builder{
		Filter: new(req.GetFilter()), MediaKind: new(pluginKinds[req.GetMediaKind()]), Query: new(req.GetQuery()),
		Ids: req.GetIds(), Language: new(req.GetLanguage()), Region: new(req.GetRegion()),
	}.Build()
	values, err := s.requests.FilterValues(ctx, req.GetCatalog(), pr)
	if err != nil {
		return nil, requestError(ctx, err)
	}
	return acquisitionv1.ListFilterValuesResponse_builder{Values: values}.Build(), nil
}

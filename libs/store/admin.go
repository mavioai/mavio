package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/activity"
	"github.com/mavioai/mavio/libs/store/internal/ent/apikey"
	"github.com/mavioai/mavio/libs/store/internal/ent/job"
	"github.com/mavioai/mavio/libs/store/internal/ent/predicate"
	"github.com/mavioai/mavio/libs/store/internal/ent/setting"
)

// serverSettingsKey keys the server settings among the settings rows.
const serverSettingsKey = "server"

type settings struct{ s *Store }

// Settings returns the server settings.
func (s *Store) Settings() core.SettingsRepository { return settings{s} }

func (r settings) Get(ctx context.Context) (core.ServerSettings, error) {
	row, err := r.s.read.Setting.Query().Where(setting.Key(serverSettingsKey)).Only(ctx)
	if ent.IsNotFound(err) {
		return core.DefaultServerSettings(), nil
	}
	if err != nil {
		return core.ServerSettings{}, mapErr(err, "get settings")
	}
	// Settings added later keep their defaults.
	out := core.DefaultServerSettings()
	if err := json.Unmarshal([]byte(row.Value), &out); err != nil {
		return core.ServerSettings{}, fmt.Errorf("get settings: %w", err)
	}
	out.UpdatedAt = row.UpdatedAt.UTC()
	return out, nil
}

func (r settings) Put(ctx context.Context, s *core.ServerSettings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	s.UpdatedAt = time.Now().UTC()
	v := *s
	v.UpdatedAt = time.Time{}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = r.s.write.Setting.Create().
		SetKey(serverSettingsKey).
		SetValue(string(data)).
		SetUpdatedAt(s.UpdatedAt).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put settings")
}

type apiKeys struct{ s *Store }

// APIKeys returns the integrations' API keys.
func (s *Store) APIKeys() core.APIKeyRepository { return apiKeys{s} }

func (r apiKeys) Create(ctx context.Context, k *core.APIKey) error {
	if k.ID.IsZero() {
		k.ID = core.NewID()
	}
	if err := k.Validate(); err != nil {
		return err
	}
	created, err := r.s.write.APIKey.Create().
		SetID(k.ID).
		SetUserID(k.UserID).
		SetName(k.Name).
		SetTokenHash(k.TokenHash).
		SetCreatedAt(orNow(k.CreatedAt)).
		Save(ctx)
	if err != nil {
		return mapErr(err, "create API key")
	}
	*k = toAPIKey(created)
	return nil
}

func (r apiKeys) GetByTokenHash(ctx context.Context, hash []byte) (core.APIKey, error) {
	k, err := r.s.read.APIKey.Query().Where(apikey.TokenHash(hash)).Only(ctx)
	if err != nil {
		return core.APIKey{}, mapErr(err, "get API key")
	}
	return toAPIKey(k), nil
}

func (r apiKeys) List(ctx context.Context) ([]core.APIKey, error) {
	keys, err := r.s.read.APIKey.Query().Order(ent.Desc(apikey.FieldCreatedAt), ent.Asc(apikey.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list API keys")
	}
	out := make([]core.APIKey, len(keys))
	for i, k := range keys {
		out[i] = toAPIKey(k)
	}
	return out, nil
}

func (r apiKeys) Touch(ctx context.Context, id core.ID, at time.Time) error {
	return mapErr(r.s.write.APIKey.UpdateOneID(id).SetLastUsedAt(at).Exec(ctx), "touch API key")
}

func (r apiKeys) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.APIKey.DeleteOneID(id).Exec(ctx), "delete API key")
}

func toAPIKey(k *ent.APIKey) core.APIKey {
	out := core.APIKey{ID: k.ID, UserID: k.UserID, Name: k.Name, TokenHash: k.TokenHash, CreatedAt: k.CreatedAt.UTC()}
	if k.LastUsedAt != nil {
		t := k.LastUsedAt.UTC()
		out.LastUsedAt = &t
	}
	return out
}

type activities struct{ s *Store }

// Activities returns the activity log.
func (s *Store) Activities() core.ActivityRepository { return activities{s} }

func (r activities) Add(ctx context.Context, a *core.Activity) error {
	if a.ID.IsZero() {
		a.ID = core.NewID()
	}
	if a.Time.IsZero() {
		a.Time = time.Now().UTC()
	}
	if err := a.Validate(); err != nil {
		return err
	}
	c := r.s.write.Activity.Create().
		SetID(a.ID).
		SetTime(a.Time).
		SetType(a.Type).
		SetSeverity(string(a.Severity)).
		SetTitle(a.Title).
		SetMessage(a.Message).
		SetAttributes(a.Attributes)
	if !a.UserID.IsZero() {
		c.SetUserID(a.UserID)
	}
	if !a.ItemID.IsZero() {
		c.SetItemID(a.ItemID)
	}
	return mapErr(c.Exec(ctx), "add activity")
}

// severitiesFrom lists the severities at least as severe as min.
func severitiesFrom(min core.Severity) []string {
	var out []string
	for _, s := range []core.Severity{core.SeverityInfo, core.SeverityWarning, core.SeverityError} {
		if s.AtLeast(min) {
			out = append(out, string(s))
		}
	}
	return out
}

func (r activities) List(ctx context.Context, q core.ActivityQuery) (core.Page[core.Activity], error) {
	if err := q.Validate(); err != nil {
		return core.Page[core.Activity]{}, err
	}
	var where []predicate.Activity
	if !q.Since.IsZero() {
		where = append(where, activity.TimeGTE(q.Since))
	}
	if !q.UserID.IsZero() {
		where = append(where, activity.UserID(q.UserID))
	}
	if q.MinSeverity != "" {
		where = append(where, activity.SeverityIn(severitiesFrom(q.MinSeverity)...))
	}
	query := r.s.read.Activity.Query().Where(where...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.Activity]{}, mapErr(err, "count activities")
	}
	rows, err := query.Order(ent.Desc(activity.FieldTime), ent.Desc(activity.FieldID)).
		Limit(q.PageSize()).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.Activity]{}, mapErr(err, "list activities")
	}
	out := core.Page[core.Activity]{Total: total, Items: make([]core.Activity, len(rows))}
	for i, a := range rows {
		out.Items[i] = core.Activity{
			ID: a.ID, Time: a.Time.UTC(), Type: a.Type, Severity: core.Severity(a.Severity), Title: a.Title,
			Message: a.Message, UserID: a.UserID, ItemID: a.ItemID, Attributes: a.Attributes,
		}
	}
	return out, nil
}

func (r activities) Purge(ctx context.Context, before time.Time) (int, error) {
	n, err := r.s.write.Activity.Delete().Where(activity.TimeLT(before)).Exec(ctx)
	return n, mapErr(err, "purge activities")
}

func (r jobs) List(ctx context.Context, q core.JobQuery) (core.Page[core.Job], error) {
	if err := q.Validate(); err != nil {
		return core.Page[core.Job]{}, err
	}
	var where []predicate.Job
	if len(q.Kinds) > 0 {
		where = append(where, job.KindIn(q.Kinds...))
	}
	if len(q.States) > 0 {
		states := make([]string, len(q.States))
		for i, s := range q.States {
			states[i] = string(s)
		}
		where = append(where, job.StateIn(states...))
	}
	query := r.s.read.Job.Query().Where(where...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.Job]{}, mapErr(err, "count jobs")
	}
	rows, err := query.Order(ent.Desc(job.FieldCreatedAt), ent.Desc(job.FieldID)).Limit(q.PageSize()).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.Job]{}, mapErr(err, "list jobs")
	}
	out := core.Page[core.Job]{Total: total, Items: make([]core.Job, len(rows))}
	for i, j := range rows {
		out.Items[i] = core.Job{
			ID: j.ID, Kind: j.Kind, Payload: j.Payload, UniqueKey: j.UniqueKey, State: core.JobState(j.State),
			Priority: j.Priority, Attempts: j.Attempts, MaxAttempts: j.MaxAttempts, RunAt: j.RunAt.UTC(),
			LeaseOwner: j.LeaseOwner, LastError: j.LastError, CreatedAt: j.CreatedAt.UTC(),
		}
		if j.LeaseExpiresAt != nil {
			out.Items[i].LeaseExpiresAt = j.LeaseExpiresAt.UTC()
		}
		if j.FinishedAt != nil {
			t := j.FinishedAt.UTC()
			out.Items[i].FinishedAt = &t
		}
	}
	return out, nil
}

func (r jobs) Purge(ctx context.Context, before time.Time) (int, error) {
	n, err := r.s.write.Job.Delete().
		Where(job.StateIn(string(core.JobSucceeded), string(core.JobFailed)), job.FinishedAtLT(before)).
		Exec(ctx)
	return n, mapErr(err, "purge jobs")
}

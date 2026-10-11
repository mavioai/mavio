package events

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

// SeverityToProto converts an activity's severity.
func SeverityToProto(s core.Severity) systemv1.Severity {
	switch s {
	case core.SeverityInfo:
		return systemv1.Severity_SEVERITY_INFO
	case core.SeverityWarning:
		return systemv1.Severity_SEVERITY_WARNING
	case core.SeverityError:
		return systemv1.Severity_SEVERITY_ERROR
	}
	return systemv1.Severity_SEVERITY_UNSPECIFIED
}

// ActivityToProto converts an activity.
func ActivityToProto(a *core.Activity) *systemv1.Activity {
	out := systemv1.Activity_builder{
		Id: new(a.ID.String()), Time: timestamppb.New(a.Time), Type: new(a.Type), Severity: new(SeverityToProto(a.Severity)),
		Title: new(a.Title), Message: new(a.Message), Attributes: a.Attributes,
	}.Build()
	if !a.UserID.IsZero() {
		out.SetUserId(a.UserID.String())
	}
	if !a.ItemID.IsZero() {
		out.SetItemId(a.ItemID.String())
	}
	return out
}

// Notify pushes a notification, such as a request approved or an issue
// reported, to the devices of the user it concerns, its UserID, when
// user, and of the administrators, when admins, as Seerr's web push
// agent tells users and administrators.
func (h *Hub) Notify(a core.Activity, user, admins bool) {
	e := sessionv1.Event_builder{Notification: sessionv1.NotificationReceived_builder{Activity: ActivityToProto(&a)}.Build()}.Build()
	for _, s := range h.subscribers(func(s *Subscriber) bool {
		return user && !a.UserID.IsZero() && s.User.ID == a.UserID || admins && s.User.Admin
	}) {
		h.send(s, e)
	}
}

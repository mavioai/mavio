package events

import (
	"context"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
)

// wants reports whether plugins take events of a type.
func (h *Hub) wants(eventType string) bool {
	return h.cfg.Events != nil && h.cfg.Events.Wants(eventType)
}

func (h *Hub) publish(a core.Activity) {
	if h.cfg.Events != nil {
		h.cfg.Events.Publish(a)
	}
}

// itemEvent describes an added, updated or removed item.
func itemEvent(eventType string, id, libraryID core.ID, name string) core.Activity {
	if name == "" {
		name = id.String()
	}
	return core.Activity{
		Type: eventType, ItemID: id, Title: name, Attributes: map[string]string{"library": libraryID.String()},
	}
}

// userDataEvent describes a user's changed state of an item; the position
// is in milliseconds.
func userDataEvent(d *core.UserData) core.Activity {
	return core.Activity{
		Type: "userdata.changed", UserID: d.UserID, ItemID: d.ItemID, Title: "User data changed",
		Attributes: map[string]string{
			"played": strconv.FormatBool(d.Played), "favorite": strconv.FormatBool(d.Favorite),
			"position": strconv.FormatInt(d.Position.Milliseconds(), 10),
		},
	}
}

// taskEnded tells plugins that a task's run ended, logging failures, and
// that a library scan ended.
func (h *Hub) taskEnded(job core.Job, cause error) {
	if h.cfg.Events == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, name := job.Kind, "Clean up jobs"
	attrs := map[string]string{}
	if libID := libraryOfJob(job); !libID.IsZero() {
		id += ":" + libID.String()
		attrs["library"] = libID.String()
		lib, err := h.cfg.Store.Libraries().Get(ctx, libID)
		if err != nil {
			lib.Name = libID.String()
		}
		name = "Scan " + lib.Name
		if job.Kind == library.JobExtras {
			name = "Make media extras of " + lib.Name
		}
	}
	if job.Kind == library.JobScan {
		scanned := core.Activity{Type: "library.scanned", Title: name, Attributes: map[string]string{
			"library": attrs["library"], "succeeded": strconv.FormatBool(cause == nil),
		}}
		if cause != nil {
			scanned.Severity, scanned.Message = core.SeverityError, cause.Error()
		}
		h.publish(scanned)
	}
	attrs["task"] = id
	if cause != nil {
		h.cfg.Events.Record(ctx, core.Activity{
			Type: "task.failed", Severity: core.SeverityError, Title: name + " failed", Message: cause.Error(), Attributes: attrs,
		})
		return
	}
	h.publish(core.Activity{Type: "task.completed", Title: name + " completed", Attributes: attrs})
}

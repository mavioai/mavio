package app

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// backupsState is the state of the server's backups.
type backupsState struct {
	list     task[[]*systemv1.Backup]
	creating bool
	busy     bool
	err      error // of making a backup
	// removing is the backup asked to be removed; removeErr why it was
	// not.
	removing   *systemv1.Backup
	confirming bool
	removeErr  error
}

func (a *App) backupService(id string) systemv1connect.BackupServiceClient {
	return systemv1connect.NewBackupServiceClient(a.connectHTTP(), serverURL(id))
}

// backupsSection builds the server's backups, as the web UI's: making
// one, and those made, newest first, to remove.
func (a *App) backupsSection(c *ui.Context) {
	st := &a.settingsState.admin.backups
	sectionHeader(c, style.T(c, "settings.backups"), style.T(c, "backups.message"))
	t := adminTask(a, &st.list, "backups", func(ctx context.Context, s string) ([]*systemv1.Backup, error) {
		res, err := a.backupService(s).ListBackups(ctx, &systemv1.ListBackupsRequest{})
		return res.GetBackups(), err
	})
	server := a.chosenServer().GetId()
	group(c, "create", "", style.T(c, "backups.restoreFooter"), func(g *settingsGroup) {
		detail, label := style.T(c, "backups.createDetail"), style.T(c, "backups.createAction")
		if st.creating {
			detail, label = style.T(c, "backups.creating"), "…"
		}
		g.row(style.T(c, "backups.create"), detail, nil, func() {
			if smallButton(c, label, st.creating) {
				st.creating, st.err = true, nil
				client := a.backupService(server)
				go func() {
					_, err := client.CreateBackup(a.ctx, &systemv1.CreateBackupRequest{})
					a.opts.Update(func() {
						st.creating, st.err = false, err
						if err == nil {
							a.settingsState.admin.reload++
						}
					})
				}()
			}
		})
		if st.err != nil {
			g.note(errorMessage(c, st.err), true)
		}
	})
	group(c, "backups", style.T(c, "backups.list"), "", func(g *settingsGroup) {
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(t.value) == 0:
			g.note(style.T(c, "backups.none"), false)
		}
		for _, b := range t.value {
			var parts []string
			if b.HasCreateTime() {
				parts = append(parts, fromNow(c, b.GetCreateTime().AsTime()))
			}
			parts = append(parts, sizeOf(b.GetSizeBytes()))
			if v := b.GetVersion(); v != "" {
				parts = append(parts, style.T(c, "backups.version", "version", v))
			}
			g.row(b.GetName(), strings.Join(parts, " · "), nil, func() {
				if dangerButton(c.Key(b.GetName()), style.T(c, "backups.remove"), st.busy) {
					st.removing, st.confirming, st.removeErr = b, true, nil
				}
			})
		}
	})
	if st.removing == nil {
		return
	}
	message := style.T(c, "backups.removeMessage")
	if st.removeErr != nil {
		message = errorMessage(c, st.removeErr)
	}
	b := st.removing
	if ui.AlertDialog(c, &st.confirming, style.T(c, "backups.removeTitle", "name", b.GetName()), message,
		style.T(c, "action.cancel"), style.T(c, "backups.remove")) == 1 {
		name := b.GetName()
		st.busy = true
		client := a.backupService(server)
		go func() {
			_, err := client.DeleteBackup(a.ctx, systemv1.DeleteBackupRequest_builder{Name: &name}.Build())
			a.opts.Update(func() {
				st.busy, st.removeErr = false, err
				if err == nil {
					a.settingsState.admin.reload++
				} else {
					// The dialog stays, telling why.
					st.confirming = true
				}
			})
		}()
	}
	if !st.confirming && !st.busy {
		st.removing = nil
	}
}

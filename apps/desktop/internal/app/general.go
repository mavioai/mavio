package app

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// serverSettingsState is the state of the sections of the server's
// settings: General, API Keys and Transcoding.
type serverSettingsState struct {
	settings    task[*systemv1.GeneralSettings]
	transcode   task[*playbackv1.GetTranscodingSettingsResponse]
	outbound    task[*systemv1.OutboundProxies]
	catalogs    task[[]string]
	plugins     task[[]*systemv1.Plugin]
	keys        task[[]*authv1.ApiKey]
	general     generalForm
	transcoding transcodingForm
	apiKeys     apiKeysForm
	busy        string // the change being made
}

// generalDraft is the General section as edited.
type generalDraft struct {
	name, language, country string
	idleDays                float64
	resetPlugin             string
	baseURL                 string
	httpsPort               float64
	certificate, key        string
	discovery, relay        bool
}

// generalForm is the General section's draft, as loaded and as edited.
type generalForm struct {
	from         *systemv1.GeneralSettings // the settings drafted from
	draft, saved generalDraft
	notice       string
	err          error
}

// transcodingDraft is the Transcoding section as edited: the fields of
// TranscodingSettings, numbers as the inputs edit them.
type transcodingDraft struct {
	acceleration              playbackv1.HardwareAcceleration
	hardwareEncoding          bool
	preset                    string
	avcCRF, hevcCRF, threads  float64
	tonemap, tonemapRange     string
	tonemapDesat, tonemapPeak float64
	deinterlace               string
	doubleRate, crop          bool
	downmixBoost              float64
	folder                    string
}

// transcodingForm is the Transcoding section's draft.
type transcodingForm struct {
	from         *playbackv1.TranscodingSettings
	draft, saved transcodingDraft
	notice       string
	err          error
}

// apiKeysForm is the API Keys section's state: the name of the key to
// make, the key made, whose token shows once, and the key being revoked.
type apiKeysForm struct {
	name                   string
	madeID, madeName, made string // the token made
	copied                 bool
	revoking               *authv1.ApiKey
	confirming             bool
	err                    error
}

func (a *App) authService(id string) authv1connect.AuthServiceClient {
	return authv1connect.NewAuthServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) pluginService(id string) systemv1connect.PluginManagementServiceClient {
	return systemv1connect.NewPluginManagementServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) playbackService(id string) playbackv1connect.PlaybackServiceClient {
	return playbackv1connect.NewPlaybackServiceClient(a.connectHTTP(), serverURL(id))
}

// generalSettings loads the general settings of the server chosen.
func (a *App) generalSettings() *task[*systemv1.GeneralSettings] {
	ss := &a.settingsState.admin.server
	return adminTask(a, &ss.settings, "general-settings", func(ctx context.Context, s string) (*systemv1.GeneralSettings, error) {
		res, err := a.systemService(s).GetGeneralSettings(ctx, &systemv1.GetGeneralSettingsRequest{})
		return res.GetSettings(), err
	})
}

// transcodingSettings loads the transcoding settings of the server
// chosen, with the accelerations its ffmpeg supports.
func (a *App) transcodingSettings() *task[*playbackv1.GetTranscodingSettingsResponse] {
	ss := &a.settingsState.admin.server
	return adminTask(a, &ss.transcode, "transcoding-settings", func(ctx context.Context, s string) (*playbackv1.GetTranscodingSettingsResponse, error) {
		return a.playbackService(s).GetTranscodingSettings(ctx, &playbackv1.GetTranscodingSettingsRequest{})
	})
}

// saveSettings runs save, which replaces a group of the server's
// settings, in the background; done takes its error on the main thread.
func (a *App) saveSettings(save func(ctx context.Context, server string) error, done func(err error)) {
	ss := &a.settingsState.admin.server
	ss.busy = "save"
	server := a.chosenServer().GetId()
	go func() {
		err := save(a.ctx, server)
		a.opts.Update(func() {
			ss.busy = ""
			if err == nil {
				a.settingsState.admin.reload++
			}
			done(err)
		})
	}()
}

// saveRow builds a section's Save, its notice once saved and the error
// of saving.
func (a *App) saveRow(c *ui.Context, changed bool, notice string, err error, save func()) {
	busy := a.settingsState.admin.server.busy != ""
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		label := style.T(c, "action.save")
		if busy {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!changed || busy).Clicked() {
			save()
		}
		if notice != "" && !changed {
			secondaryText(c, style.T(c, notice))
		}
		if err != nil {
			dangerText(c, errorMessage(c, err))
		}
	})
}

func generalDraftOf(s *systemv1.GeneralSettings) generalDraft {
	n := s.GetNetwork()
	return generalDraft{
		name: n.GetServerName(), language: s.GetMetadata().GetPreferredLanguage(), country: s.GetMetadata().GetCountry(),
		idleDays: float64(s.GetSessionIdleDays()), resetPlugin: s.GetPasswordResetPlugin(),
		baseURL: n.GetBaseUrl(), httpsPort: float64(n.GetHttpsPort()), certificate: n.GetCertificatePath(), key: n.GetKeyPath(),
		discovery: n.GetLocalDiscovery(), relay: n.GetRelay(),
	}
}

// generalSection builds the server's name, the language and country of
// its metadata, how long devices stay signed in, the plugin that resets
// forgotten passwords, and how it is reached.
func (a *App) generalSection(c *ui.Context) {
	ss := &a.settingsState.admin.server
	f := &ss.general
	sectionHeader(c, style.T(c, "settings.general"), "")
	t := a.generalSettings()
	if taskStatus(c, t, func() { ss.settings.key = "" }, nil) {
		return
	}
	if s := t.value; s != f.from && (f.from == nil || f.draft == f.saved) {
		f.from, f.draft, f.saved = s, generalDraftOf(s), generalDraftOf(s)
	}
	d := &f.draft
	server := a.chosenServer().GetId()
	mavioDev := a.account != nil
	text := func(value *string, label, detail, placeholder string) func(g *settingsGroup) {
		return func(g *settingsGroup) {
			g.row(label, detail, nil, func() {
				ui.TextInput(c, value).Width(288).Label(label).Placeholder(placeholder)
			})
		}
	}
	group(c, "general", "", "", func(g *settingsGroup) {
		text(&d.name, style.T(c, "general.name"), style.T(c, "general.nameDetail"), "")(g)
		g.row(style.T(c, "general.language"), style.T(c, "general.languageDetail"), nil, func() {
			codes, labels := []string{""}, []string{style.T(c, "general.languageDefault")}
			for _, l := range a.languages(server) {
				if l.GetCode() != "" && !slices.Contains(codes, l.GetCode()) {
					codes, labels = append(codes, l.GetCode()), append(labels, cmpOr(languageName(c, l.GetCode()), l.GetName()))
				}
			}
			if !slices.Contains(codes, d.language) {
				codes, labels = append(codes, d.language), append(labels, languageName(c, d.language))
			}
			i := slices.Index(codes, d.language)
			if choose(c, style.T(c, "general.language"), &i, labels) {
				d.language = codes[i]
			}
		})
		g.row(style.T(c, "general.country"), style.T(c, "general.countryDetail"), nil, func() {
			codes, labels := []string{""}, []string{style.T(c, "general.countryDefault")}
			for _, x := range a.countries(server) {
				if x.GetCode() != "" && !slices.Contains(codes, x.GetCode()) {
					codes, labels = append(codes, x.GetCode()), append(labels, regionName(c, x.GetCode()))
				}
			}
			if !slices.Contains(codes, d.country) {
				codes, labels = append(codes, d.country), append(labels, regionName(c, d.country))
			}
			i := slices.Index(codes, d.country)
			if choose(c, style.T(c, "general.country"), &i, labels) {
				d.country = codes[i]
			}
		})
	})
	group(c, "sign-in", style.T(c, "general.signIn"), "", func(g *settingsGroup) {
		g.row(style.T(c, "general.idleDays"), style.T(c, "general.idleDaysDetail"), nil, func() {
			ui.NumberInput(c, &d.idleDays, 1, 3650, 1).Label(style.T(c, "general.idleDays"))
		})
		if mavioDev {
			return
		}
		plugins := adminTask(a, &ss.plugins, "plugins", func(ctx context.Context, s string) ([]*systemv1.Plugin, error) {
			res, err := a.pluginService(s).ListPlugins(ctx, &systemv1.ListPluginsRequest{})
			return res.GetPlugins(), err
		})
		ids, labels := []string{""}, []string{style.T(c, "general.resetOff")}
		for _, p := range plugins.value {
			m := p.GetManifest()
			if slices.Contains(m.GetCapabilities(), pluginv1.Capability_CAPABILITY_PASSWORD_RESET) {
				ids, labels = append(ids, m.GetId()), append(labels, cmpOr(m.GetName(), m.GetId()))
			}
		}
		detail := style.T(c, "general.resetPluginDetail")
		if len(ids) == 1 {
			detail = style.T(c, "general.resetPluginNone")
		}
		if !slices.Contains(ids, d.resetPlugin) {
			ids, labels = append(ids, d.resetPlugin), append(labels, d.resetPlugin)
		}
		g.row(style.T(c, "general.resetPlugin"), detail, nil, func() {
			i := slices.Index(ids, d.resetPlugin)
			if choose(c, style.T(c, "general.resetPlugin"), &i, labels) {
				d.resetPlugin = ids[i]
			}
		})
	})
	group(c, "network", style.T(c, "general.network"), style.T(c, "general.networkFooter"), func(g *settingsGroup) {
		text(&d.baseURL, style.T(c, "general.baseUrl"), style.T(c, "general.baseUrlDetail"), "/mavio")(g)
		detail := style.T(c, "general.httpsPortDetail")
		if mavioDev {
			detail = style.T(c, "general.httpsPortMavioDev")
		}
		g.row(style.T(c, "general.httpsPort"), detail, nil, func() {
			ui.NumberInput(c, &d.httpsPort, 0, 65535, 1).Label(style.T(c, "general.httpsPort"))
		})
		if d.httpsPort > 0 {
			text(&d.certificate, style.T(c, "general.certificate"), style.T(c, "general.certificateDetail"), "/etc/mavio/cert.pem")(g)
			text(&d.key, style.T(c, "general.key"), "", "/etc/mavio/key.pem")(g)
		}
		g.switchRow(style.T(c, "general.discovery"), style.T(c, "general.discoveryDetail"), &d.discovery)
		if mavioDev {
			g.switchRow(style.T(c, "general.relay"), style.T(c, "general.relayDetail"), &d.relay)
		}
	})
	changed := *d != f.saved
	a.saveRow(c, changed, f.notice, f.err, func() {
		draft := *d
		f.err = nil
		settings := systemv1.GeneralSettings_builder{
			Network: systemv1.NetworkSettings_builder{
				ServerName: new(strings.TrimSpace(draft.name)), BaseUrl: new(strings.TrimSpace(draft.baseURL)), HttpsPort: new(int32(draft.httpsPort)),
				CertificatePath: new(strings.TrimSpace(draft.certificate)), KeyPath: new(strings.TrimSpace(draft.key)),
				LocalDiscovery: new(draft.discovery), Relay: new(draft.relay),
			}.Build(),
			Metadata:            systemv1.MetadataSettings_builder{PreferredLanguage: new(draft.language), Country: new(draft.country)}.Build(),
			SessionIdleDays:     new(int32(draft.idleDays)),
			PasswordResetPlugin: new(draft.resetPlugin),
		}.Build()
		a.saveSettings(func(ctx context.Context, server string) error {
			_, err := a.systemService(server).UpdateGeneralSettings(ctx, systemv1.UpdateGeneralSettingsRequest_builder{Settings: settings}.Build())
			return err
		}, func(err error) {
			f.err = err
			if err == nil {
				f.saved, f.notice = draft, "general.saved"
			}
		})
	})
}

// accelerationTitles name the hardware accelerations.
var accelerationTitles = map[playbackv1.HardwareAcceleration]string{
	playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_AUTO:         "transcoding.accelerationAuto",
	playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_NONE:         "transcoding.accelerationNone",
	playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_VIDEOTOOLBOX: "VideoToolbox",
}

var (
	encoderPresets = []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"}
	tonemaps       = []string{"bt2390", "hable", "mobius", "reinhard", "clip", "linear", "gamma"}
	tonemapRanges  = []string{"auto", "tv", "pc"}
	deinterlacers  = []string{"yadif", "bwdif"}
)

func transcodingDraftOf(t *playbackv1.TranscodingSettings) transcodingDraft {
	return transcodingDraft{
		acceleration: t.GetHardwareAcceleration(), hardwareEncoding: t.GetHardwareEncoding(), preset: t.GetEncoderPreset(),
		avcCRF: float64(t.GetH264Crf()), hevcCRF: float64(t.GetH265Crf()), threads: float64(t.GetThreads()),
		tonemap: t.GetTonemapAlgorithm(), tonemapRange: t.GetTonemapRange(), tonemapDesat: t.GetTonemapDesat(), tonemapPeak: t.GetTonemapPeak(),
		deinterlace: t.GetDeinterlaceMethod(), doubleRate: t.GetDeinterlaceDoubleRate(), crop: t.GetCropBlackBorders(),
		downmixBoost: t.GetDownmixBoost(), folder: t.GetTranscodeDir(),
	}
}

// settings returns the TranscodingSettings of a draft.
func (d transcodingDraft) settings() *playbackv1.TranscodingSettings {
	return playbackv1.TranscodingSettings_builder{
		HardwareAcceleration: new(d.acceleration), HardwareEncoding: new(d.hardwareEncoding), EncoderPreset: new(d.preset),
		H264Crf: new(int32(d.avcCRF)), H265Crf: new(int32(d.hevcCRF)), Threads: new(int32(d.threads)),
		TonemapAlgorithm: new(d.tonemap), TonemapRange: new(d.tonemapRange), TonemapDesat: new(d.tonemapDesat), TonemapPeak: new(d.tonemapPeak),
		DeinterlaceMethod: new(d.deinterlace), DeinterlaceDoubleRate: new(d.doubleRate), CropBlackBorders: new(d.crop),
		DownmixBoost: new(d.downmixBoost), TranscodeDir: new(strings.TrimSpace(d.folder)),
	}.Build()
}

// chooseString builds a choice among values, with labels of their own
// or the values themselves; a value not offered is added.
func chooseString(c *ui.Context, label string, value *string, values, labels []string) {
	values, labels = slices.Clone(values), slices.Clone(labels)
	if labels == nil {
		labels = slices.Clone(values)
	}
	if !slices.Contains(values, *value) {
		values, labels = append(values, *value), append(labels, *value)
	}
	i := slices.Index(values, *value)
	if choose(c, label, &i, labels) {
		*value = values[i]
	}
}

// transcodingSection builds how the server re-encodes what devices
// cannot play.
func (a *App) transcodingSection(c *ui.Context) {
	ss := &a.settingsState.admin.server
	f := &ss.transcoding
	sectionHeader(c, style.T(c, "settings.transcoding"), style.T(c, "transcoding.message"))
	t := a.transcodingSettings()
	if taskStatus(c, t, func() { ss.transcode.key = "" }, nil) {
		return
	}
	if s := t.value.GetSettings(); s != f.from && (f.from == nil || f.draft == f.saved) {
		f.from, f.draft, f.saved = s, transcodingDraftOf(s), transcodingDraftOf(s)
	}
	d := &f.draft
	number := func(g *settingsGroup, value *float64, label, detail string, lo, hi, step float64) {
		g.row(label, detail, nil, func() {
			ui.NumberInput(c, value, lo, hi, step).Label(label)
		})
	}
	group(c, "hardware", style.T(c, "transcoding.hardware"), "", func(g *settingsGroup) {
		g.row(style.T(c, "transcoding.acceleration"), style.T(c, "transcoding.accelerationDetail"), nil, func() {
			offered := slices.Clone(t.value.GetAvailableHardwareAccelerations())
			if len(offered) == 0 {
				offered = []playbackv1.HardwareAcceleration{playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_AUTO, playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_NONE}
			}
			if !slices.Contains(offered, d.acceleration) && d.acceleration != playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_UNSPECIFIED {
				offered = append(offered, d.acceleration)
			}
			labels := make([]string, len(offered))
			for i, x := range offered {
				labels[i] = strconv.Itoa(int(x))
				if title, ok := accelerationTitles[x]; ok {
					labels[i] = title
					if strings.HasPrefix(title, "transcoding.") {
						labels[i] = style.T(c, title)
					}
				}
			}
			i := slices.Index(offered, d.acceleration)
			if choose(c, style.T(c, "transcoding.acceleration"), &i, labels) {
				d.acceleration = offered[i]
			}
		})
		if d.acceleration != playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_NONE {
			g.switchRow(style.T(c, "transcoding.hardwareEncoding"), style.T(c, "transcoding.hardwareEncodingDetail"), &d.hardwareEncoding)
		}
	})
	group(c, "encoding", style.T(c, "transcoding.encoding"), "", func(g *settingsGroup) {
		g.row(style.T(c, "transcoding.preset"), style.T(c, "transcoding.presetDetail"), nil, func() {
			chooseString(c, style.T(c, "transcoding.preset"), &d.preset, append([]string{""}, encoderPresets...),
				append([]string{style.T(c, "transcoding.presetAuto")}, encoderPresets...))
		})
		number(g, &d.avcCRF, style.T(c, "transcoding.avcCrf"), style.T(c, "transcoding.crfDetail"), 0, 51, 1)
		number(g, &d.hevcCRF, style.T(c, "transcoding.hevcCrf"), "", 0, 51, 1)
		number(g, &d.threads, style.T(c, "transcoding.threads"), style.T(c, "transcoding.threadsDetail"), 0, 256, 1)
	})
	group(c, "video", style.T(c, "transcoding.video"), "", func(g *settingsGroup) {
		g.row(style.T(c, "transcoding.tonemap"), style.T(c, "transcoding.tonemapDetail"), nil, func() {
			chooseString(c, style.T(c, "transcoding.tonemap"), &d.tonemap, tonemaps, nil)
		})
		g.row(style.T(c, "transcoding.tonemapRange"), "", nil, func() {
			chooseString(c, style.T(c, "transcoding.tonemapRange"), &d.tonemapRange, tonemapRanges, nil)
		})
		number(g, &d.tonemapDesat, style.T(c, "transcoding.tonemapDesat"), "", 0, 10, 0.1)
		number(g, &d.tonemapPeak, style.T(c, "transcoding.tonemapPeak"), style.T(c, "transcoding.tonemapPeakDetail"), 0, 10000, 1)
		g.row(style.T(c, "transcoding.deinterlace"), "", nil, func() {
			chooseString(c, style.T(c, "transcoding.deinterlace"), &d.deinterlace, deinterlacers, nil)
		})
		g.switchRow(style.T(c, "transcoding.doubleRate"), style.T(c, "transcoding.doubleRateDetail"), &d.doubleRate)
		g.switchRow(style.T(c, "transcoding.crop"), style.T(c, "transcoding.cropDetail"), &d.crop)
	})
	group(c, "audio", style.T(c, "transcoding.audio"), "", func(g *settingsGroup) {
		number(g, &d.downmixBoost, style.T(c, "transcoding.downmixBoost"), style.T(c, "transcoding.downmixBoostDetail"), 0.5, 3, 0.1)
	})
	group(c, "folder", style.T(c, "transcoding.folder"), style.T(c, "transcoding.folderFooter"), func(g *settingsGroup) {
		g.row(style.T(c, "transcoding.folder"), "", nil, func() {
			ui.TextInput(c, &d.folder).Width(288).Label(style.T(c, "transcoding.folder")).Placeholder(style.T(c, "transcoding.folderDefault"))
		})
	})
	changed := *d != f.saved
	a.saveRow(c, changed, f.notice, f.err, func() {
		draft := *d
		f.err = nil
		a.saveSettings(func(ctx context.Context, server string) error {
			_, err := a.playbackService(server).UpdateTranscodingSettings(ctx, playbackv1.UpdateTranscodingSettingsRequest_builder{Settings: draft.settings()}.Build())
			return err
		}, func(err error) {
			f.err = err
			if err == nil {
				f.saved, f.notice = draft, "transcoding.saved"
			}
		})
	})
}

// apiKeysSection builds the keys integrations call the server's API with:
// making one, whose token shows once, and revoking them.
func (a *App) apiKeysSection(c *ui.Context) {
	ss := &a.settingsState.admin.server
	f := &ss.apiKeys
	sectionHeader(c, style.T(c, "settings.apiKeys"), style.T(c, "keys.message"))
	t := adminTask(a, &ss.keys, "api-keys", func(ctx context.Context, s string) ([]*authv1.ApiKey, error) {
		res, err := a.authService(s).ListApiKeys(ctx, &authv1.ListApiKeysRequest{})
		return res.GetApiKeys(), err
	})
	server := a.chosenServer().GetId()
	action := func(fn func(ctx context.Context, s authv1connect.AuthServiceClient) error, done func(err error)) {
		ss.busy = "keys"
		client := a.authService(server)
		go func() {
			err := fn(a.ctx, client)
			a.opts.Update(func() {
				ss.busy = ""
				if err == nil {
					a.settingsState.admin.reload++
				}
				done(err)
			})
		}()
	}
	footer := style.T(c, "keys.createFooter")
	if f.err != nil {
		footer = errorMessage(c, f.err)
	}
	create := func() {
		name := strings.TrimSpace(f.name)
		var res *authv1.CreateApiKeyResponse
		f.err = nil
		action(func(ctx context.Context, s authv1connect.AuthServiceClient) (err error) {
			res, err = s.CreateApiKey(ctx, authv1.CreateApiKeyRequest_builder{Name: &name}.Build())
			return err
		}, func(err error) {
			f.err = err
			if err == nil {
				f.madeID, f.madeName, f.made, f.copied, f.name = res.GetApiKey().GetId(), cmpOr(res.GetApiKey().GetName(), name), res.GetToken(), false, ""
			}
		})
	}
	group(c, "create", "", footer, func(g *settingsGroup) {
		g.block(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &f.name).Grow(1).Label(style.T(c, "keys.name")).Placeholder(style.T(c, "keys.name"))
				valid := strings.TrimSpace(f.name) != "" && len([]rune(strings.TrimSpace(f.name))) <= 100
				label := style.T(c, "keys.create")
				if ss.busy == "keys" {
					label = "…"
				}
				if (ui.PrimaryButton(c, label).Disabled(!valid || ss.busy != "").Clicked() || in.Submitted()) && valid && ss.busy == "" {
					create()
				}
			})
		})
	})
	if taskStatus(c, t, func() { ss.keys.key = "" }, nil) {
		return
	}
	if f.made != "" && slices.ContainsFunc(t.value, func(k *authv1.ApiKey) bool { return k.GetId() == f.madeID }) {
		group(c, "made", style.T(c, "keys.madeTitle", "name", f.madeName), style.T(c, "keys.madeFooter"), func(g *settingsGroup) {
			g.block(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					style.Text(ui.Text(c, f.made), style.Type(c).Callout).Font("SF Mono, Menlo, monospace").Grow(1).Shrink(1).Selectable().
						TextColor(style.Color(style.Colors(c).TextPrimary))
					label := style.T(c, "keys.copy")
					if f.copied {
						label = style.T(c, "users.copied")
					}
					if smallButton(c, label, false) {
						c.WriteClipboard(f.made)
						f.copied = true
					}
					if linkButton(c, style.T(c, "keys.done")).Clicked() {
						f.made = ""
					}
				})
			})
		})
	}
	names := map[string]string{}
	for _, u := range a.users().value {
		names[u.GetId()] = u.GetName()
	}
	keys := slices.Clone(t.value)
	group(c, "keys", style.T(c, "keys.list"), "", func(g *settingsGroup) {
		for _, k := range keys {
			var parts []string
			if n := names[k.GetUserId()]; n != "" {
				parts = append(parts, style.T(c, "keys.actsAs", "name", n))
			}
			if k.HasCreateTime() {
				parts = append(parts, style.T(c, "keys.created", "when", fromNow(c, k.GetCreateTime().AsTime())))
			}
			if k.HasLastUseTime() {
				parts = append(parts, style.T(c, "keys.used", "when", fromNow(c, k.GetLastUseTime().AsTime())))
			} else {
				parts = append(parts, style.T(c, "keys.neverUsed"))
			}
			g.row(k.GetName(), strings.Join(parts, " · "), nil, func() {
				if dangerButton(c.Key(k.GetId()), style.T(c, "keys.revoke"), ss.busy != "") {
					f.revoking, f.confirming = k, true
				}
			})
		}
		if len(keys) == 0 {
			g.note(style.T(c, "keys.none"), false)
		}
	})
	if f.revoking == nil {
		return
	}
	title := style.T(c, "keys.revokeTitle", "name", f.revoking.GetName())
	if ui.AlertDialog(c, &f.confirming, title, style.T(c, "keys.revokeMessage"), style.T(c, "action.cancel"), style.T(c, "keys.revoke")) == 1 {
		id := f.revoking.GetId()
		f.err = nil
		action(func(ctx context.Context, s authv1connect.AuthServiceClient) error {
			_, err := s.RevokeApiKey(ctx, authv1.RevokeApiKeyRequest_builder{Id: &id}.Build())
			return err
		}, func(err error) { f.err = err })
	}
	if !f.confirming {
		f.revoking = nil
	}
}

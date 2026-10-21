package app

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// aiState is the state of the settings' AI (docs/clients.md §6.3.13).
type aiState struct {
	providers task[[]*pluginv1.AiProvider]
	// models are those of the provider whose page shows.
	models task[[]*pluginv1.AiModel]

	// The server's assistant settings: those loaded, the draft of them
	// and the settings it started from, and its saving.
	assistant task[*assistantv1.GetAssistantSettingsResponse]
	draft     *assistantv1.AssistantSettings
	draftOf   *assistantv1.AssistantSettings
	// numbers are the texts typed in its fields of numbers.
	numbers map[string]string
	saving  bool
	saved   bool
	saveErr error

	// A provider's page: its API key typed, its saving and signing out.
	apiKey     string
	keySaved   bool
	keyBusy    bool
	keyErr     error
	signingOut bool
	signOutErr error
	confirming bool
	login      aiLogin
	copied     string

	conv convState
}

// aiLogin is a provider's OAuth login, followed as it runs.
type aiLogin struct {
	provider, id string
	state        *systemv1.WatchAiLoginResponse
	err          error
	starting     bool
	answering    bool
	stop         context.CancelFunc
	// answer is the value typed or chosen for the prompt promptID.
	answer, promptID string
}

// isNoAiGateway reports whether an error says that no AI gateway plugin
// runs.
func isNoAiGateway(err error) bool { return connect.CodeOf(err) == connect.CodeFailedPrecondition }

func aiPath(provider string) string {
	if provider == "" {
		return "/settings/ai"
	}
	return "/settings/ai?" + url.Values{"id": {provider}}.Encode()
}

// aiSection builds the settings' AI, as the web UI's: the assistant's
// settings and the providers, or a provider's page.
func (a *App) aiSection(c *ui.Context, r *ui.Route) {
	if id := r.Query("id"); id != "" {
		a.aiProviderPage(c, id)
		return
	}
	if id := r.Query("conversation"); id != "" {
		a.conversationPage(c, id)
		return
	}
	a.aiProviders(c)
}

func (a *App) aiProviderList() *task[[]*pluginv1.AiProvider] {
	st := &a.settingsState.admin.ai
	return adminTask(a, &st.providers, "ai-providers", func(ctx context.Context, s string) ([]*pluginv1.AiProvider, error) {
		res, err := a.aiProviderService(s).ListAiProviders(ctx, &systemv1.ListAiProvidersRequest{})
		return res.GetProviders(), err
	})
}

// providerNames are the names of the AI providers by their IDs.
func (a *App) providerNames() map[string]string {
	names := map[string]string{}
	for _, p := range a.aiProviderList().value {
		names[p.GetId()] = p.GetName()
	}
	return names
}

// credentialOf is where a provider's credential comes from, or that it has
// none.
func credentialOf(c *ui.Context, p *pluginv1.AiProvider) string {
	switch {
	case !p.GetAvailable():
		return style.T(c, "ai.notSignedIn")
	case p.GetCredential() == pluginv1.AiCredential_AI_CREDENTIAL_OAUTH:
		return style.T(c, "ai.signedIn", "label", cmpOr(p.GetOauthLabel(), p.GetName()))
	case p.GetCredential() == pluginv1.AiCredential_AI_CREDENTIAL_API_KEY:
		return style.T(c, "ai.apiKeyStored")
	}
	return style.T(c, "ai.fromSource", "source", p.GetSource())
}

func (a *App) aiProviders(c *ui.Context) {
	sectionHeader(c, style.T(c, "settings.ai"), style.T(c, "ai.message"))
	providers := a.aiProviderList()
	a.assistantServerSettings(c)
	if isNoAiGateway(providers.err) {
		group(c, "no-gateway", "", style.T(c, "ai.noGatewayFooter"), func(g *settingsGroup) {
			if g.link("plugins", "plugins", ui.RGB(0xAF, 0x52, 0xDE), style.T(c, "ai.noGateway"), style.T(c, "ai.noGatewayDetail"), true) {
				a.router.Push(pluginsPath("", false))
			}
		})
		return
	}
	models := a.aiModels()
	byName := func(x, y *pluginv1.AiProvider) int {
		return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
	}
	var added, others []*pluginv1.AiProvider
	for _, p := range providers.value {
		if p.GetAvailable() {
			added = append(added, p)
		} else {
			others = append(others, p)
		}
	}
	slices.SortFunc(added, byName)
	slices.SortFunc(others, byName)
	group(c, "providers", style.T(c, "ai.providers"), style.T(c, "ai.providersFooter"), func(g *settingsGroup) {
		for _, p := range added {
			n := 0
			for _, m := range models.value {
				if m.GetProvider() == p.GetId() {
					n++
				}
			}
			detail := []string{credentialOf(c, p), style.T(c, "ai.modelCount", "count", n)}
			if p.GetCustom() {
				detail = append(detail, style.T(c, "ai.custom"))
			}
			if g.link("provider-"+p.GetId(), "key", ui.RGB(0x34, 0xC7, 0x59), p.GetName(), strings.Join(detail, " · "), true) {
				a.router.Push(aiPath(p.GetId()))
			}
		}
		if providers.loaded && providers.err == nil && len(added) == 0 {
			g.note(style.T(c, "ai.noProviders"), false)
		}
		if providers.err != nil {
			g.note(errorMessage(c, providers.err), true)
		}
		if len(others) > 0 {
			g.row(style.T(c, "ai.addProvider"), "", nil, func() {
				options := make([]modelOption, 0, len(others))
				for _, p := range others {
					var ways []string
					if p.GetApiKey() {
						ways = append(ways, style.T(c, "ai.apiKey"))
					}
					if p.GetOauth() {
						ways = append(ways, style.T(c, "ai.oauth"))
					}
					if p.GetCustom() {
						ways = append(ways, style.T(c, "ai.custom"))
					}
					options = append(options, modelOption{value: p.GetId(), label: p.GetName(), detail: strings.Join(ways, " · "), provider: p.GetId()})
				}
				chosen := ""
				if modelPicker(c, style.T(c, "ai.addProvider"), style.T(c, "ai.chooseProvider"), &chosen, options, false, 256) {
					a.router.Push(aiPath(chosen))
				}
			})
		}
	})
	a.aiConversations(c)
}

func (a *App) aiProviderPage(c *ui.Context, id string) {
	st := &a.settingsState.admin.ai
	colors := style.Colors(c)
	a.backTo(c, style.T(c, "settings.ai"), aiPath(""))
	providers := a.aiProviderList()
	i := slices.IndexFunc(providers.value, func(p *pluginv1.AiProvider) bool { return p.GetId() == id })
	if i < 0 {
		sectionHeader(c, id, "")
		switch {
		case providers.err != nil:
			dangerText(c, errorMessage(c, providers.err))
		case providers.loaded:
			style.Text(ui.Text(c, style.T(c, "ai.providerGone")), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary))
		}
		return
	}
	p := providers.value[i]
	if st.login.provider != id {
		a.stopLogin()
		st.login = aiLogin{provider: id}
		st.apiKey, st.keySaved, st.keyErr, st.signOutErr = "", false, nil, nil
	}
	sectionHeader(c, p.GetName(), credentialOf(c, p))
	if p.GetOauth() {
		a.oauthLogin(c, p)
	}
	if p.GetApiKey() {
		a.apiKeyGroup(c, p)
	}
	if p.GetCredential() != pluginv1.AiCredential_AI_CREDENTIAL_UNSPECIFIED {
		a.aiSignOut(c, p)
	}
	a.providerModels(c, p)
}

func (a *App) apiKeyGroup(c *ui.Context, p *pluginv1.AiProvider) {
	st := &a.settingsState.admin.ai
	stored := p.GetCredential() == pluginv1.AiCredential_AI_CREDENTIAL_API_KEY
	footer := style.T(c, "ai.apiKeyFooter")
	if stored {
		footer = style.T(c, "ai.apiKeyReplaceFooter")
	}
	group(c, "api-key", style.T(c, "ai.apiKey"), footer, func(g *settingsGroup) {
		g.row(style.T(c, "ai.apiKey"), "", nil, func() {
			in := ui.TextInput(c.Key("api-key"), &st.apiKey).Password().Width(288).Label(style.T(c, "ai.apiKey"))
			if stored {
				in.Placeholder("••••••••")
			}
			if in.Changed() {
				st.keySaved = false
			}
		})
		g.block(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
				label := style.T(c, "action.save")
				if st.keyBusy {
					label = "…"
				}
				key := strings.TrimSpace(st.apiKey)
				if ui.PrimaryButton(c, label).Disabled(key == "" || st.keyBusy).Clicked() {
					provider := p.GetId()
					st.keyBusy, st.keyErr = true, nil
					a.aiAction(func(ctx context.Context, s systemv1connect.AiProviderServiceClient) error {
						_, err := s.SetAiApiKey(ctx, systemv1.SetAiApiKeyRequest_builder{Provider: &provider, Key: &key}.Build())
						return err
					}, func(err error) {
						st.keyBusy, st.keyErr = false, err
						if err == nil {
							st.apiKey, st.keySaved = "", true
						}
					})
				}
				if st.keySaved {
					secondaryText(c, style.T(c, "ai.apiKeySaved"))
				}
				if st.keyErr != nil {
					dangerText(c, errorMessage(c, st.keyErr))
				}
			})
		})
	})
}

// aiAction runs a change of the AI providers in the background; done
// takes its result on the main thread, and the providers and models load
// again.
func (a *App) aiAction(fn func(ctx context.Context, s systemv1connect.AiProviderServiceClient) error, done func(err error)) {
	s := a.aiProviderService(a.chosenServer().GetId())
	go func() {
		err := fn(a.ctx, s)
		a.opts.Update(func() {
			done(err)
			a.settingsState.admin.reload++
		})
	}()
}

func (a *App) aiSignOut(c *ui.Context, p *pluginv1.AiProvider) {
	st := &a.settingsState.admin.ai
	action := style.T(c, "ai.removeKey")
	if p.GetCredential() == pluginv1.AiCredential_AI_CREDENTIAL_OAUTH {
		action = style.T(c, "ai.signOut")
	}
	ui.Row(c).Children(func() {
		if dangerButton(c, action, st.signingOut) {
			st.confirming = true
		}
	})
	if st.signOutErr != nil {
		dangerText(c, errorMessage(c, st.signOutErr))
	}
	if ui.AlertDialog(c, &st.confirming, action, style.T(c, "ai.signOutFooter"), style.T(c, "action.cancel"), action) == 1 {
		provider := p.GetId()
		st.signingOut, st.signOutErr = true, nil
		a.aiAction(func(ctx context.Context, s systemv1connect.AiProviderServiceClient) error {
			_, err := s.LogoutAi(ctx, systemv1.LogoutAiRequest_builder{Provider: &provider}.Build())
			return err
		}, func(err error) { st.signingOut, st.signOutErr = false, err })
	}
}

// stopLogin stops following the login under way, if any.
func (a *App) stopLogin() {
	if l := &a.settingsState.admin.ai.login; l.stop != nil {
		l.stop()
		l.stop = nil
	}
}

// startLogin starts a provider's OAuth login and follows it until it
// ends, the providers and models loading again then.
func (a *App) startLogin(provider string) {
	st := &a.settingsState.admin.ai
	a.stopLogin()
	st.login = aiLogin{provider: provider, starting: true}
	s := a.aiProviderService(a.chosenServer().GetId())
	ctx, stop := context.WithCancel(a.ctx)
	st.login.stop = stop
	mine := func() bool { return st.login.provider == provider && st.login.stop != nil && ctx.Err() == nil }
	go func() {
		res, err := s.StartAiLogin(ctx, systemv1.StartAiLoginRequest_builder{Provider: &provider}.Build())
		id := res.GetLoginId()
		a.opts.Update(func() {
			if !mine() {
				return
			}
			st.login.starting, st.login.err, st.login.id = false, err, id
		})
		if err != nil {
			return
		}
		stream, err := s.WatchAiLogin(ctx, systemv1.WatchAiLoginRequest_builder{LoginId: &id}.Build())
		if err == nil {
			for stream.Receive() {
				m := stream.Msg()
				a.opts.Update(func() {
					if mine() {
						st.login.state = m
					}
				})
			}
			err = stream.Err()
			stream.Close()
		}
		a.opts.Update(func() {
			if ctx.Err() != nil {
				return
			}
			if mine() && err != nil {
				st.login.err = err
			}
			a.settingsState.admin.reload++
		})
	}()
}

// oauthLogin builds a provider's OAuth login, as the web UI's: started
// here, followed as it shows pages and codes and asks questions, until it
// ends.
func (a *App) oauthLogin(c *ui.Context, p *pluginv1.AiProvider) {
	st := &a.settingsState.admin.ai
	l := &st.login
	state := l.state.GetState()
	running := l.id != "" && (l.state == nil || state == pluginv1.AiLoginState_AI_LOGIN_STATE_RUNNING) && l.err == nil
	label := cmpOr(p.GetOauthLabel(), p.GetName())
	footer := ""
	if p.GetSubscription() {
		footer = style.T(c, "ai.subscriptionFooter")
	}
	group(c, "oauth", style.T(c, "ai.oauth"), footer, func(g *settingsGroup) {
		if !running {
			detail := ""
			if p.GetCredential() == pluginv1.AiCredential_AI_CREDENTIAL_OAUTH {
				detail = style.T(c, "ai.signInAgain")
			}
			g.row(style.T(c, "ai.signInWith", "label", label), detail, nil, func() {
				text := style.T(c, "ai.signIn")
				if l.starting {
					text = "…"
				}
				if smallButton(c, text, l.starting) {
					a.startLogin(p.GetId())
				}
			})
		}
		if l.id != "" {
			for i, e := range l.state.GetEvents() {
				a.loginEvent(g, i, e)
			}
		}
		if pr := l.state.GetPrompt(); running && pr != nil {
			a.loginPrompt(g, pr)
		}
		if running {
			g.block(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					secondaryText(c, style.T(c, "ai.waiting"))
					if smallButton(c, style.T(c, "action.cancel"), false) {
						id := l.id
						a.stopLogin()
						*l = aiLogin{provider: p.GetId()}
						a.aiAction(func(ctx context.Context, s systemv1connect.AiProviderServiceClient) error {
							_, err := s.CancelAiLogin(ctx, systemv1.CancelAiLoginRequest_builder{LoginId: &id}.Build())
							return err
						}, func(err error) {
							if err != nil && l.provider == p.GetId() {
								l.err = err
							}
						})
					}
				})
			})
		}
		switch state {
		case pluginv1.AiLoginState_AI_LOGIN_STATE_SUCCEEDED:
			g.note(style.T(c, "ai.loginSucceeded"), false)
		case pluginv1.AiLoginState_AI_LOGIN_STATE_FAILED:
			g.note(cmpOr(l.state.GetError(), style.T(c, "ai.loginFailed")), true)
		case pluginv1.AiLoginState_AI_LOGIN_STATE_CANCELED:
			g.note(style.T(c, "ai.loginCanceled"), false)
		}
		if l.err != nil {
			g.note(errorMessage(c, l.err), true)
		}
	})
}

// loginEvent builds what a login shows: a message with its links, a page
// to open, a code to enter on a page, or its progress.
func (a *App) loginEvent(g *settingsGroup, i int, e *pluginv1.AiLoginEvent) {
	c := g.c
	colors := style.Colors(c)
	link := func(k, label, href string) {
		b := ui.ButtonBase(c.Key(k)).AlignSelf(ui.Start).Label(label).Cursor(ui.CursorPointer)
		b.Children(func() {
			t := style.Text(ui.Text(c, label), style.Type(c).Callout).TextColor(style.Color(colors.AccentText))
			if b.Hovered() {
				t.Underline()
			}
		})
		if b.Clicked() {
			_ = a.opts.OpenURL(href)
		}
	}
	g.divider()
	ui.Column(c.Key(key("event", i))).Padding(10, 16).Gap(6).Children(func() {
		switch {
		case e.HasInfo():
			info := e.GetInfo()
			style.Text(ui.Text(c, info.GetMessage()), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
			for j, l := range info.GetLinks() {
				link(key("link", j), cmpOr(l.GetLabel(), l.GetUrl()), l.GetUrl())
			}
		case e.HasUrl():
			u := e.GetUrl()
			if u.GetInstructions() != "" {
				style.Text(ui.Text(c, u.GetInstructions()), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
			}
			ui.Row(c).Children(func() {
				if ui.Button(c, style.T(c, "ai.openPage")).Clicked() {
					_ = a.opts.OpenURL(u.GetUrl())
				}
			})
			a.copyableURL(c, u.GetUrl())
		case e.HasDeviceCode():
			d := e.GetDeviceCode()
			style.Text(ui.Text(c, style.T(c, "ai.enterCode")), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary))
			ui.Box(c).AlignSelf(ui.Start).Padding(6, 12).Radius(6).Background(style.Color(colors.SurfaceRaised)).Children(func() {
				style.Text(ui.Text(c, d.GetUserCode()), style.Type(c).Title3).Font("SF Mono, Menlo, monospace").LetterSpacing(2).
					TextColor(style.Color(colors.TextPrimary)).Selectable()
			})
			link("verify", d.GetVerificationUri(), d.GetVerificationUri())
		case e.HasProgress():
			style.Text(ui.Text(c, e.GetProgress()), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
		}
	})
}

// copyableURL builds a long address on one line, cut to fit, with a
// button copying it.
func (a *App) copyableURL(c *ui.Context, u string) {
	st := &a.settingsState.admin.ai
	colors := style.Colors(c)
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		style.Text(ui.Text(c, u), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().
			Grow(1).Shrink(1).MinWidth(0).Tooltip(u)
		icon, label := "copy", style.T(c, "ai.copyLink")
		if st.copied == u {
			icon, label = "check", style.T(c, "users.copied")
		}
		b := ui.ButtonBase(c.Key("copy-link")).Size(32, 32).Shrink(0).Radius(8).Center().Label(style.T(c, "ai.copyLink")).Tooltip(label)
		fg := colors.TextSecondary
		if b.Hovered() {
			b.Background(style.Color(colors.Fill))
			fg = colors.TextPrimary
		}
		b.Children(func() { ui.Icon(c, style.Icon(icon, false)).Size(16, 16).TextColor(style.Color(fg)) })
		if b.Clicked() {
			c.WriteClipboard(u)
			st.copied = u
		}
	})
}

// loginPrompt builds a login's question: text, a secret, a choice, or the
// code of a page whose redirect could not reach the plugin.
func (a *App) loginPrompt(g *settingsGroup, pr *pluginv1.AiLoginPrompt) {
	c := g.c
	l := &a.settingsState.admin.ai.login
	colors := style.Colors(c)
	kind := pr.GetKind()
	if l.promptID != pr.GetId() {
		l.promptID, l.answer = pr.GetId(), ""
		if kind == pluginv1.AiLoginPromptKind_AI_LOGIN_PROMPT_KIND_SELECT && len(pr.GetOptions()) > 0 {
			l.answer = pr.GetOptions()[0].GetId()
		}
	}
	label := cmpOr(pr.GetMessage(), style.T(c, "ai.answer"))
	g.divider()
	ui.Column(c.Key("prompt-"+pr.GetId())).Padding(10, 16).Gap(8).Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Selectable()
		if kind == pluginv1.AiLoginPromptKind_AI_LOGIN_PROMPT_KIND_SELECT {
			var ids, labels []string
			for _, o := range pr.GetOptions() {
				ids = append(ids, o.GetId())
				if o.GetDescription() != "" {
					labels = append(labels, o.GetLabel()+" — "+o.GetDescription())
				} else {
					labels = append(labels, o.GetLabel())
				}
			}
			ui.Row(c).Children(func() { chooseString(c, label, &l.answer, ids, labels) })
		} else {
			in := ui.TextInput(c.Key("answer"), &l.answer).Label(label).Placeholder(pr.GetPlaceholder())
			if kind == pluginv1.AiLoginPromptKind_AI_LOGIN_PROMPT_KIND_SECRET {
				in.Password()
			}
		}
		if kind == pluginv1.AiLoginPromptKind_AI_LOGIN_PROMPT_KIND_MANUAL_CODE {
			style.Text(ui.Text(c, style.T(c, "ai.manualCode")), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
		}
		ui.Row(c).Children(func() {
			text := style.T(c, "ai.submit")
			if l.answering {
				text = "…"
			}
			value := strings.TrimSpace(l.answer)
			if ui.PrimaryButton(c, text).Disabled(value == "" || l.answering).Clicked() {
				id, prompt, provider := l.id, pr.GetId(), l.provider
				l.answering, l.err = true, nil
				a.aiAction(func(ctx context.Context, s systemv1connect.AiProviderServiceClient) error {
					_, err := s.AnswerAiLogin(ctx, systemv1.AnswerAiLoginRequest_builder{LoginId: &id, PromptId: &prompt, Value: &value}.Build())
					return err
				}, func(err error) {
					if l.provider == provider {
						l.answering, l.err = false, err
					}
				})
			}
		})
	})
}

func (a *App) providerModels(c *ui.Context, p *pluginv1.AiProvider) {
	st := &a.settingsState.admin.ai
	colors := style.Colors(c)
	id := p.GetId()
	models := adminTask(a, &st.models, "ai-provider-models", func(ctx context.Context, s string) ([]*pluginv1.AiModel, error) {
		res, err := a.aiProviderService(s).ListAiModels(ctx, systemv1.ListAiModelsRequest_builder{Provider: &id}.Build())
		return res.GetModels(), err
	}, id)
	footer := ""
	if !p.GetAvailable() {
		footer = style.T(c, "ai.modelsFooter")
	}
	group(c, "models", style.T(c, "ai.models"), footer, func(g *settingsGroup) {
		if len(models.value) > 0 {
			g.divider()
			// Providers list up to hundreds of models; the list scrolls
			// within the group.
			ui.Scroll(c.Key("models")).MaxHeight(448).Children(func() {
				ui.Column(c).Children(func() {
					for i, m := range models.value {
						if i > 0 {
							ui.Box(c).Height(1).Margin(0, 0, 0, 16).Background(style.Color(colors.Border))
						}
						ui.Column(c.Key(m.GetId())).Padding(10, 16).Gap(2).Children(func() {
							style.Text(ui.Text(c, cmpOr(m.GetName(), m.GetId())), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
							if d := modelDetail(c, m); d != "" {
								style.Text(ui.Text(c, d), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Selectable()
							}
						})
					}
				})
			})
		}
		if models.loaded && models.err == nil && len(models.value) == 0 {
			g.note(style.T(c, "ai.noModels"), false)
		}
		if models.err != nil {
			g.note(errorMessage(c, models.err), true)
		}
	})
}

// modelDetail is a model's details on one line, as the web UI's: its ID
// when it has a name, its context window, reasoning, images and cost.
func modelDetail(c *ui.Context, m *pluginv1.AiModel) string {
	var parts []string
	if m.GetName() != "" && m.GetId() != m.GetName() {
		parts = append(parts, m.GetId())
	}
	if w := m.GetContextWindow(); w > 0 {
		parts = append(parts, style.T(c, "ai.contextWindow", "tokens", compactNumber(w)))
	}
	if m.GetReasoning() {
		parts = append(parts, style.T(c, "ai.reasoning"))
	}
	if m.GetImages() {
		parts = append(parts, style.T(c, "ai.images"))
	}
	if cost := m.GetCost(); cost.GetInput() > 0 || cost.GetOutput() > 0 {
		parts = append(parts, style.T(c, "ai.cost", "input", dollars(cost.GetInput()), "output", dollars(cost.GetOutput())))
	}
	return strings.Join(parts, " · ")
}

// compactNumber is a count in the compact notation, as 128K or 1M: two
// significant digits under a hundred of the unit.
func compactNumber(n int64) string {
	units := []struct {
		size   float64
		suffix string
	}{{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "K"}}
	for _, u := range units {
		if float64(n) >= u.size {
			v := float64(n) / u.size
			if v < 100 {
				p := math.Pow(10, 1-math.Floor(math.Log10(v)))
				v = math.Round(v*p) / p
			} else {
				v = math.Round(v)
			}
			return strconv.FormatFloat(v, 'f', -1, 64) + u.suffix
		}
	}
	return strconv.FormatInt(n, 10)
}

func dollars(v float64) string { return fmt.Sprintf("$%.2f", v) }

// assistantServerSettings builds the server's assistant settings, in its
// AI settings, as the web UI's (docs/assistant.md §10).
func (a *App) assistantServerSettings(c *ui.Context) {
	st := &a.settingsState.admin.ai
	res := adminTask(a, &st.assistant, "assistant-settings", func(ctx context.Context, s string) (*assistantv1.GetAssistantSettingsResponse, error) {
		return a.assistantService(s).GetAssistantSettings(ctx, &assistantv1.GetAssistantSettingsRequest{})
	})
	if res.err != nil {
		dangerText(c, errorMessage(c, res.err))
		return
	}
	settings := res.value.GetSettings()
	if settings == nil {
		return
	}
	if st.draftOf != settings {
		st.draftOf, st.draft, st.numbers = settings, proto.CloneOf(settings), map[string]string{}
	}
	d := st.draft
	t := func(k string) string { return style.T(c, k) }
	off := !d.GetEnabled()
	changed := !proto.Equal(d, settings)
	switchRow := func(g *settingsGroup, label, detail string, on bool, disabled bool, set func(bool)) {
		if g.switchRowOf(label, detail, &on, disabled) {
			set(on)
			st.saved = false
		}
	}
	group(c, "assistant", t("assistant.server"), "", func(g *settingsGroup) {
		switchRow(g, t("assistant.enabled"), t("assistant.enabledDetail"), d.GetEnabled(), false, d.SetEnabled)
		g.row(t("assistant.model"), "", nil, func() {
			options := append([]modelOption{{label: t("assistant.noModel")}}, modelOptions(c, "ai/", a.aiModels().value, a.providerNames())...)
			model := d.GetModel()
			if modelPicker(c, t("assistant.model"), t("assistant.noModel"), &model, options, off, 288) {
				d.SetModel(model)
				st.saved = false
			}
		})
		switchRow(g, t("assistant.codeMode"), t("assistant.codeModeDetail"), d.GetCodeMode(), off || d.GetModel() == "", d.SetCodeMode)
		g.row(t("assistant.aiAccess"), "", nil, func() {
			values := []assistantv1.AssistantAccess{
				assistantv1.AssistantAccess_ASSISTANT_ACCESS_ALL,
				assistantv1.AssistantAccess_ASSISTANT_ACCESS_ADMINISTRATORS,
				assistantv1.AssistantAccess_ASSISTANT_ACCESS_SELECTED,
			}
			i := max(0, slices.Index(values, d.GetAiAccess()))
			labels := []string{t("assistant.accessAll"), t("assistant.accessAdmins"), t("assistant.accessSelected")}
			chosen := labels[i]
			if selectOf(c, &chosen, labels).MinWidth(200).Label(t("assistant.aiAccess")).Disabled(off || d.GetModel() == "").Changed() {
				d.SetAiAccess(values[slices.Index(labels, chosen)])
				st.saved = false
			}
		})
		if d.GetAiAccess() == assistantv1.AssistantAccess_ASSISTANT_ACCESS_SELECTED {
			g.divider()
			ui.Row(c.Key("ai-users")).Padding(12, 16).GapX(20).GapY(8).Wrap().Children(func() {
				for _, u := range a.users().value {
					on := slices.Contains(d.GetAiUserIds(), u.GetId())
					if ui.Checkbox(c.Key("user-"+u.GetId()), &on, u.GetName()).Changed() {
						ids := slices.DeleteFunc(slices.Clone(d.GetAiUserIds()), func(x string) bool { return x == u.GetId() })
						if on {
							ids = append(ids, u.GetId())
						}
						d.SetAiUserIds(ids)
						st.saved = false
					}
				}
			})
		}
		g.row(t("assistant.idleMinutes"), "", nil, func() {
			optionalNumber(c, st.numbers, t("assistant.idleMinutes"), "30", float64(d.GetIdleMinutes()), 1, 1440, true, false, func(v float64) {
				d.SetIdleMinutes(int32(v))
				st.saved = false
			})
		})
		g.row(t("assistant.retentionDays"), "", nil, func() {
			optionalNumber(c, st.numbers, t("assistant.retentionDays"), "30", float64(d.GetRetentionDays()), 1, 3650, true, false, func(v float64) {
				d.SetRetentionDays(int32(v))
				st.saved = false
			})
		})
		switchRow(g, t("assistant.serverMemory"), t("assistant.serverMemoryDetail"), d.GetMemory(), off, d.SetMemory)
		switchRow(g, t("assistant.hints"), t("assistant.hintsDetail"), d.GetHints(), off, d.SetHints)
	})
	a.assistantSpeech(c, d, res.value, off)
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		label := t("action.save")
		if st.saving {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!changed || st.saving).Clicked() {
			draft := proto.CloneOf(d)
			server := a.chosenServer().GetId()
			st.saving, st.saveErr = true, nil
			go func() {
				_, err := a.assistantService(server).UpdateAssistantSettings(a.ctx, assistantv1.UpdateAssistantSettingsRequest_builder{Settings: draft}.Build())
				a.opts.Update(func() {
					st.saving, st.saveErr, st.saved = false, err, err == nil
					if err == nil {
						a.settingsState.admin.reload++
					}
				})
			}()
		}
		if changed && ui.Button(c, t("action.cancel")).Clicked() {
			st.draft, st.numbers = proto.CloneOf(settings), map[string]string{}
		}
		if st.saved && !changed {
			secondaryText(c, t("assistant.saved"))
		}
		if st.saveErr != nil {
			dangerText(c, errorMessage(c, st.saveErr))
		}
	})
}

// assistantSpeech builds the recognizer and voice of each language and the
// timing of turns.
func (a *App) assistantSpeech(c *ui.Context, d *assistantv1.AssistantSettings, res *assistantv1.GetAssistantSettingsResponse, off bool) {
	st := &a.settingsState.admin.ai
	t := func(k string) string { return style.T(c, k) }
	engines := func(es []*assistantv1.VoiceEngine) (ids, labels []string) {
		ids, labels = []string{""}, []string{t("assistant.firstReady")}
		for _, e := range es {
			ids, labels = append(ids, e.GetId()), append(labels, cmpOr(e.GetName(), e.GetId()))
		}
		return ids, labels
	}
	recognizers, recognizerLabels := engines(res.GetRecognizers())
	voices, voiceLabels := engines(res.GetVoices())
	set := func(i int, fn func(s *assistantv1.AssistantSpeech)) {
		speech := slices.Clone(d.GetSpeech())
		s := proto.CloneOf(speech[i])
		fn(s)
		speech[i] = s
		d.SetSpeech(speech)
		st.saved = false
	}
	group(c, "speech", t("assistant.speech"), t("assistant.speechFooter"), func(g *settingsGroup) {
		for i, s := range d.GetSpeech() {
			g.divider()
			ui.Row(c.Key(key("speech", i))).Padding(10, 16).Gap(8).AlignItems(ui.Center).Children(func() {
				lang := s.GetLanguage()
				if ui.TextInput(c.Key("language"), &lang).Width(96).Placeholder("en").Label(t("assistant.language")).Disabled(off).Changed() {
					set(i, func(s *assistantv1.AssistantSpeech) { s.SetLanguage(strings.TrimSpace(lang)) })
				}
				ui.Box(c.Key("recognizer")).Grow(1).MinWidth(0).Children(func() {
					v := s.GetRecognizer()
					chooseString(c, t("assistant.recognizer"), &v, recognizers, recognizerLabels)
					if v != s.GetRecognizer() && !off {
						set(i, func(s *assistantv1.AssistantSpeech) { s.SetRecognizer(v) })
					}
				})
				ui.Box(c.Key("voice")).Grow(1).MinWidth(0).Children(func() {
					v := s.GetVoice()
					chooseString(c, t("assistant.voice"), &v, voices, voiceLabels)
					if v != s.GetVoice() && !off {
						set(i, func(s *assistantv1.AssistantSpeech) { s.SetVoice(v) })
					}
				})
				if dangerButton(c, t("action.delete"), false) {
					d.SetSpeech(slices.Delete(slices.Clone(d.GetSpeech()), i, i+1))
					st.saved = false
				}
			})
		}
		if len(res.GetRecognizers()) == 0 && len(res.GetVoices()) == 0 {
			if g.link("voice-plugins", "plugins", ui.RGB(0xAF, 0x52, 0xDE), t("assistant.noVoicePlugin"), t("ai.noGatewayDetail"), true) {
				a.router.Push(pluginsPath("", false))
			}
		}
		g.block(func() {
			ui.Row(c).Children(func() {
				if ui.Button(c, t("assistant.addLanguage")).Disabled(off).Clicked() {
					lang, _, _ := strings.Cut(c.Locale(), "-")
					d.SetSpeech(append(slices.Clone(d.GetSpeech()), assistantv1.AssistantSpeech_builder{Language: &lang}.Build()))
					st.saved = false
				}
			})
		})
		durationRow := func(label, placeholder string, v *durationpb.Duration, lo, hi float64, integer bool, setD func(*durationpb.Duration)) {
			g.row(label, "", nil, func() {
				secs := 0.0
				if v != nil {
					secs = v.AsDuration().Seconds()
				}
				optionalNumber(c, st.numbers, label, placeholder, secs, lo, hi, integer, off, func(s float64) {
					if s == 0 {
						setD(nil)
					} else {
						setD(durationpb.New(time.Duration(math.Round(s*1000)) * time.Millisecond))
					}
					st.saved = false
				})
			})
		}
		durationRow(t("assistant.longestPause"), "3", d.GetLongestPause(), 0.5, 10, false, d.SetLongestPause)
		durationRow(t("assistant.commitDelay"), "0.7", d.GetCommitDelay(), 0, 3, false, d.SetCommitDelay)
		durationRow(t("assistant.sessionIdle"), "8", d.GetSessionIdle(), 3, 120, true, d.SetSessionIdle)
	})
}

// optionalNumber builds a field of a number between lo and hi, empty for
// 0, which leaves the server's default, placeholder. What is typed stays
// in texts, by label, until it is a number in range, which set takes.
func optionalNumber(c *ui.Context, texts map[string]string, label, placeholder string, value, lo, hi float64, integer, disabled bool, set func(float64)) {
	text, typing := texts[label]
	if !typing && value != 0 {
		text = strconv.FormatFloat(value, 'f', -1, 64)
	}
	in := ui.TextInput(c.Key(label), &text).Width(128).Placeholder(placeholder).Label(label).Disabled(disabled)
	if !in.Changed() {
		return
	}
	texts[label] = text
	t := strings.TrimSpace(text)
	if t == "" {
		set(0)
		return
	}
	n, err := strconv.ParseFloat(t, 64)
	if err != nil || math.IsNaN(n) || n < lo || n > hi || (integer && n != math.Trunc(n)) {
		return
	}
	set(n)
}

package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/mavioai/mavio/libs/core"
)

// PasskeyTimeout is how long a passkey ceremony may take.
const PasskeyTimeout = 5 * time.Minute

// ErrPasskey is returned for a passkey response that does not verify,
// and for an unknown or expired ceremony.
var ErrPasskey = errors.New("passkey not accepted")

// RelyingParty is the web origin a passkey belongs to: the address a
// browser reached the server at. Its ID is the origin's host, so that a
// passkey works at the address it was created at.
type RelyingParty struct {
	ID     string
	Origin string
}

// RelyingPartyOf returns the relying party of a request's Origin header:
// an HTTPS origin, or HTTP on localhost, as browsers allow; IP addresses
// cannot be relying parties.
func RelyingPartyOf(origin string) (RelyingParty, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.Path != "" {
		return RelyingParty{}, fmt.Errorf("%w: passkeys need a web origin", core.ErrInvalid)
	}
	host := u.Hostname()
	switch {
	case net.ParseIP(host) != nil:
		return RelyingParty{}, fmt.Errorf("%w: passkeys need a host name, not an IP address", core.ErrInvalid)
	case u.Scheme != "https" && (u.Scheme != "http" || host != "localhost"):
		return RelyingParty{}, fmt.Errorf("%w: passkeys need HTTPS", core.ErrInvalid)
	}
	return RelyingParty{ID: host, Origin: u.Scheme + "://" + u.Host}, nil
}

// Passkeys runs WebAuthn ceremonies: creating a passkey for a signed-in
// user and signing in with one, which names its user itself
// (discoverable credentials). Ceremonies are kept in memory and used
// once.
type Passkeys struct {
	now func() time.Time

	mu         sync.Mutex
	ceremonies map[string]ceremony
}

type ceremony struct {
	rp      RelyingParty
	session webauthn.SessionData
	user    core.ID
	expires time.Time
}

// passkey is the data of a passkey credential.
type passkey struct {
	RPID       string              `json:"rp_id"`
	Credential webauthn.Credential `json:"credential"`
}

// NewPasskeys returns passkeys without ceremonies.
func NewPasskeys() *Passkeys {
	return &Passkeys{now: time.Now, ceremonies: map[string]ceremony{}}
}

// StartRegistration begins creating a passkey of u at rp, excluding the
// passkeys u has there, and returns the ceremony's ID and the creation
// options, as PublicKeyCredential.parseCreationOptionsFromJSON takes them.
func (p *Passkeys) StartRegistration(rp RelyingParty, u core.User, creds []core.Credential) (string, []byte, error) {
	wa, err := relyingParty(rp)
	if err != nil {
		return "", nil, err
	}
	user := passkeyUser{user: u, creds: passkeysAt(rp, creds)}
	creation, session, err := wa.BeginRegistration(user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey: protocol.ResidentKeyRequirementRequired, RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification: protocol.VerificationRequired,
		}),
		webauthn.WithExclusions(webauthn.Credentials(user.creds).CredentialDescriptors()),
	)
	if err != nil {
		return "", nil, fmt.Errorf("begin passkey registration: %w", err)
	}
	options, err := json.Marshal(creation.Response)
	if err != nil {
		return "", nil, err
	}
	return p.start(ceremony{rp: rp, session: *session, user: u.ID}), options, nil
}

// FinishRegistration verifies the response, PublicKeyCredential.toJSON,
// to a registration of user, and returns the data of the new passkey.
func (p *Passkeys) FinishRegistration(id string, user core.User, response []byte) (string, error) {
	c, ok := p.end(id)
	if !ok || c.user != user.ID {
		return "", ErrPasskey
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPasskey, err)
	}
	wa, err := relyingParty(c.rp)
	if err != nil {
		return "", err
	}
	cred, err := wa.CreateCredential(passkeyUser{user: user}, c.session, parsed)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPasskey, err)
	}
	data, err := json.Marshal(passkey{RPID: c.rp.ID, Credential: *cred})
	return string(data), err
}

// StartSignIn begins signing in with a passkey at rp and returns the
// ceremony's ID and the request options, as
// PublicKeyCredential.parseRequestOptionsFromJSON takes them.
func (p *Passkeys) StartSignIn(rp RelyingParty) (string, []byte, error) {
	wa, err := relyingParty(rp)
	if err != nil {
		return "", nil, err
	}
	assertion, session, err := wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return "", nil, fmt.Errorf("begin passkey sign-in: %w", err)
	}
	options, err := json.Marshal(assertion.Response)
	if err != nil {
		return "", nil, err
	}
	return p.start(ceremony{rp: rp, session: *session}), options, nil
}

// FinishSignIn verifies the response to a sign-in, looking up the user it
// names and its credentials, and returns the user and its passkey with
// the data to store after its use.
func (p *Passkeys) FinishSignIn(id string, response []byte, lookup func(core.ID) (core.User, []core.Credential, error)) (core.User, core.Credential, error) {
	c, ok := p.end(id)
	if !ok {
		return core.User{}, core.Credential{}, ErrPasskey
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return core.User{}, core.Credential{}, fmt.Errorf("%w: %w", ErrPasskey, err)
	}
	wa, err := relyingParty(c.rp)
	if err != nil {
		return core.User{}, core.Credential{}, err
	}
	var (
		user  core.User
		creds []core.Credential
	)
	_, cred, err := wa.ValidatePasskeyLogin(func(_, handle []byte) (webauthn.User, error) {
		if len(handle) != len(core.ID{}) {
			return nil, ErrPasskey
		}
		var err error
		if user, creds, err = lookup(core.ID(handle)); err != nil {
			return nil, err
		}
		return passkeyUser{user: user, creds: passkeysAt(c.rp, creds)}, nil
	}, c.session, parsed)
	switch {
	case errors.Is(err, core.ErrNotFound):
		return core.User{}, core.Credential{}, ErrPasskey
	case err != nil:
		var perr *protocol.Error
		if errors.As(err, &perr) || errors.Is(err, ErrPasskey) {
			return core.User{}, core.Credential{}, fmt.Errorf("%w: %w", ErrPasskey, err)
		}
		return core.User{}, core.Credential{}, err
	case cred.Authenticator.CloneWarning:
		return core.User{}, core.Credential{}, fmt.Errorf("%w: its signature counter went back", ErrPasskey)
	}
	for _, stored := range creds {
		var pk passkey
		if stored.Kind != core.CredentialPasskey || json.Unmarshal([]byte(stored.Data), &pk) != nil || !slices.Equal(pk.Credential.ID, cred.ID) {
			continue
		}
		pk.Credential = *cred
		data, err := json.Marshal(pk)
		if err != nil {
			return core.User{}, core.Credential{}, err
		}
		stored.Data = string(data)
		return user, stored, nil
	}
	return core.User{}, core.Credential{}, ErrPasskey
}

func (p *Passkeys) start(c ceremony) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	id := base64.RawURLEncoding.EncodeToString(b)
	now := p.now()
	c.expires = now.Add(PasskeyTimeout)
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, old := range p.ceremonies {
		if !now.Before(old.expires) {
			delete(p.ceremonies, k)
		}
	}
	p.ceremonies[id] = c
	return id
}

func (p *Passkeys) end(id string) (ceremony, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.ceremonies[id]
	delete(p.ceremonies, id)
	return c, ok && p.now().Before(c.expires)
}

func relyingParty(rp RelyingParty) (*webauthn.WebAuthn, error) {
	timeout := webauthn.TimeoutConfig{Enforce: true, Timeout: PasskeyTimeout, TimeoutUVD: PasskeyTimeout}
	wa, err := webauthn.New(&webauthn.Config{
		RPID: rp.ID, RPDisplayName: "Mavio", RPOrigins: []string{rp.Origin},
		Timeouts: webauthn.TimeoutsConfig{Login: timeout, Registration: timeout},
	})
	if err != nil {
		return nil, fmt.Errorf("relying party %s: %w", rp.Origin, err)
	}
	return wa, nil
}

// passkeysAt returns the WebAuthn credentials among creds that belong to
// rp.
func passkeysAt(rp RelyingParty, creds []core.Credential) []webauthn.Credential {
	var out []webauthn.Credential
	for _, c := range creds {
		var pk passkey
		if c.Kind == core.CredentialPasskey && json.Unmarshal([]byte(c.Data), &pk) == nil && pk.RPID == rp.ID {
			out = append(out, pk.Credential)
		}
	}
	return out
}

// passkeyUser is a user as WebAuthn sees it: its handle is its ID.
type passkeyUser struct {
	user  core.User
	creds []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return u.user.ID[:] }
func (u passkeyUser) WebAuthnName() string                       { return u.user.Name }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.user.Name }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

// PasskeyRPID returns the host a passkey's data belongs to; empty for
// data that is not a passkey's.
func PasskeyRPID(data string) string {
	var pk passkey
	if json.Unmarshal([]byte(data), &pk) != nil {
		return ""
	}
	return pk.RPID
}

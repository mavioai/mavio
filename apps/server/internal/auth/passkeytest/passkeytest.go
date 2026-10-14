// Package passkeytest is a software authenticator for tests: it answers
// WebAuthn creation and request options as a browser and a platform
// authenticator would, with discoverable P-256 credentials and "none"
// attestation.
package passkeytest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// Authenticator holds passkeys and answers ceremonies for one origin.
type Authenticator struct {
	origin string
	keys   []credential
}

type credential struct {
	id, user []byte
	rpID     string
	key      *ecdsa.PrivateKey
	count    uint32
}

// New returns an authenticator in a browser at origin.
func New(origin string) *Authenticator { return &Authenticator{origin: origin} }

// Origin changes the origin the authenticator's browser is at.
func (a *Authenticator) Origin(origin string) { a.origin = origin }

const (
	flagUserPresent  = 0x01
	flagUserVerified = 0x04
	flagAttested     = 0x40
)

var b64 = base64.RawURLEncoding

// Create answers creation options, as
// PublicKeyCredential.parseCreationOptionsFromJSON takes them, with the
// new credential, as PublicKeyCredential.toJSON gives it.
func (a *Authenticator) Create(options []byte) ([]byte, error) {
	var opts struct {
		Challenge string `json:"challenge"`
		RP        struct {
			ID string `json:"id"`
		} `json:"rp"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(options, &opts); err != nil {
		return nil, err
	}
	user, err := b64.DecodeString(opts.User.ID)
	if err != nil {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	c := credential{id: make([]byte, 16), user: user, rpID: opts.RP.ID, key: key}
	_, _ = rand.Read(c.id)
	a.keys = append(a.keys, c)

	pub, err := key.PublicKey.Bytes() // uncompressed: 0x04 | x | y
	if err != nil {
		return nil, err
	}
	cose, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: pub[1:33], -3: pub[33:]})
	if err != nil {
		return nil, err
	}
	authData := authenticatorData(c.rpID, flagUserPresent|flagUserVerified|flagAttested, 0)
	authData = append(authData, make([]byte, 16)...) // AAGUID
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(c.id)))
	authData = append(authData, c.id...)
	authData = append(authData, cose...)
	attestation, err := cbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": authData})
	if err != nil {
		return nil, err
	}
	clientData, err := a.clientData("webauthn.create", opts.Challenge)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"id": b64.EncodeToString(c.id), "rawId": b64.EncodeToString(c.id), "type": "public-key",
		"authenticatorAttachment": "platform", "clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"attestationObject": b64.EncodeToString(attestation), "clientDataJSON": b64.EncodeToString(clientData),
			"transports": []string{"internal"},
		},
	})
}

// Get answers request options with an assertion of the authenticator's
// newest credential for their relying party.
func (a *Authenticator) Get(options []byte) ([]byte, error) {
	var opts struct {
		Challenge string `json:"challenge"`
		RPID      string `json:"rpId"`
	}
	if err := json.Unmarshal(options, &opts); err != nil {
		return nil, err
	}
	i := len(a.keys) - 1
	for ; i >= 0 && a.keys[i].rpID != opts.RPID; i-- {
	}
	if i < 0 {
		return nil, errors.New("no credential for " + opts.RPID)
	}
	c := &a.keys[i]
	c.count++
	authData := authenticatorData(c.rpID, flagUserPresent|flagUserVerified, c.count)
	clientData, err := a.clientData("webauthn.get", opts.Challenge)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(authData, hash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, c.key, digest[:])
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"id": b64.EncodeToString(c.id), "rawId": b64.EncodeToString(c.id), "type": "public-key",
		"authenticatorAttachment": "platform", "clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"authenticatorData": b64.EncodeToString(authData), "clientDataJSON": b64.EncodeToString(clientData),
			"signature": b64.EncodeToString(sig), "userHandle": b64.EncodeToString(c.user),
		},
	})
}

func (a *Authenticator) clientData(typ, challenge string) ([]byte, error) {
	if challenge == "" {
		return nil, fmt.Errorf("%s options without a challenge", typ)
	}
	return json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": a.origin, "crossOrigin": false})
}

func authenticatorData(rpID string, flags byte, count uint32) []byte {
	hash := sha256.Sum256([]byte(rpID))
	out := append(hash[:], flags)
	return binary.BigEndian.AppendUint32(out, count)
}

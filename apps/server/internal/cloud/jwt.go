package cloud

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/identity"
)

// TokenLifetime is how long a request's JWT is valid.
const TokenLifetime = time.Minute

// Claims are the claims of the JWTs a server signs.
type Claims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	ID        string `json:"jti"`
}

// Header is the JOSE header of the JWTs a server signs.
type Header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

// Token returns a JWT for audience, mavio.dev's API base URL, signed by
// key with EdDSA (private_key_jwt): the server's ID is its kid, iss and
// sub, and it expires TokenLifetime after now.
func Token(key *identity.Key, audience string, now time.Time) string {
	id := key.ID()
	header, _ := json.Marshal(Header{Algorithm: "EdDSA", Type: "JWT", KeyID: id})
	claims, _ := json.Marshal(Claims{
		Issuer: id, Subject: id, Audience: audience,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(TokenLifetime).Unix(), ID: rand.Text(),
	})
	enc := base64.RawURLEncoding
	signed := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	return signed + "." + enc.EncodeToString(key.Sign([]byte(signed)))
}

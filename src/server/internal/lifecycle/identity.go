package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strings"
)

// Identifier formats (ADR-020): the internal ID is 32 lowercase hex
// characters; the public ID and the edit token are base62 of 128 random bits,
// fixed at 22 characters.
const (
	idBytes     = 16
	base62Width = 22
)

// Identity is the three random values of a new workout and the token's hash.
type Identity struct {
	ID        string   // internal ID, names private/{id}; the only identifier allowed in logs
	PublicID  string   // in the viewing link; never logged
	Token     string   // the edit token; only in the edit link's fragment
	TokenHash [32]byte // SHA-256 of the token's ASCII bytes; the only form stored
}

// NewIdentity reads 48 bytes from r (crypto/rand in production). A short read
// is an error.
func NewIdentity(r io.Reader) (Identity, error) {
	var b [3 * idBytes]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return Identity{}, fmt.Errorf("lifecycle: identity: %w", err)
	}
	id := Identity{
		ID:       hex.EncodeToString(b[:idBytes]),
		PublicID: base62(b[idBytes : 2*idBytes]),
		Token:    base62(b[2*idBytes:]),
	}
	id.TokenHash = HashToken(id.Token)
	return id, nil
}

// HashToken is the stored form of an edit token (ADR-020 item 4).
func HashToken(token string) [32]byte { return sha256.Sum256([]byte(token)) }

// base62 writes b as a big-endian unsigned integer in base 62 with the digit
// order of math/big (0-9, a-z, A-Z), left-padded with 0 to 22 characters.
func base62(b []byte) string {
	s := new(big.Int).SetBytes(b).Text(62)
	return strings.Repeat("0", base62Width-len(s)) + s
}

// ViewPath is the viewing link, root-relative (ADR-020 items 6, 8).
func ViewPath(publicID string) string { return "/w/" + publicID }

// EditLink is the edit link: the token goes in the fragment, which browsers
// never send (ADR-020 item 7).
func EditLink(publicID, token string) string { return "/edit/" + publicID + "#" + token }

// NewKey reads 16 bytes from r (crypto/rand in production) and returns them
// as a 22-character base62 credential with the edit token's format, hashed
// with HashToken (the report delete key, ADR-042 item 1).
func NewKey(r io.Reader) (string, error) {
	var b [idBytes]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return "", fmt.Errorf("lifecycle: key: %w", err)
	}
	return base62(b[:]), nil
}

package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// notifyStateFile holds a 16-byte salt and the 32-byte HMAC-SHA256 of the
// notification settings (F-70 NS-13). Plain text would hold a secret; the
// salted hash only lets a reader of the data directory guess a weak topic
// offline, and the originals stored there are worth more.
const notifyStateFile = "notify.state"

// notifyMAC is the HMAC over the kind, URL, token and Pushover keys; "off"
// when no service is set.
func notifyMAC(salt []byte, s settings) []byte {
	m := hmac.New(sha256.New, salt)
	if _, on := s.notifyConfig(); !on {
		m.Write([]byte("off"))
	} else {
		for _, p := range []string{string(s.notifyKind), s.notifyURL, s.notifyToken, s.pushoverToken, s.pushoverUser} {
			m.Write([]byte(p))
			m.Write([]byte{0})
		}
	}
	return m.Sum(nil)
}

// notifyChanged compares the settings with the stored value. Unchanged: a nil
// commit. Changed, or no usable file (the first start after the upgrade):
// commit writes the new value, to be called after every delete key was
// cleared, so a crash in between clears again at the next start. The error
// text carries no setting.
func notifyChanged(dataDir string, s settings) (commit func() error, err error) {
	path := filepath.Join(dataDir, notifyStateFile)
	salt := make([]byte, 16)
	old, rerr := os.ReadFile(path)
	if rerr == nil && len(old) == 48 {
		copy(salt, old[:16])
		if hmac.Equal(old[16:], notifyMAC(salt, s)) {
			return nil, nil
		}
	} else if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("notify state: %w", err)
	}
	return func() error {
		b := append(append([]byte(nil), salt...), notifyMAC(salt, s)...)
		tmp := path + ".new"
		if err := os.WriteFile(tmp, b, 0o600); err != nil {
			return fmt.Errorf("notify state: %w", err)
		}
		if err := os.Rename(tmp, path); err != nil {
			return fmt.Errorf("notify state: %w", err)
		}
		return nil
	}, nil
}

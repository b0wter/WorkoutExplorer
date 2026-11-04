package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"workout-explorer/internal/about"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/pushover"
)

// The settings read from the environment (Technical Design, Conventions:
// Configuration); nothing else in the binary reads it.
const (
	envMaxConcurrentUploads = "WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS"
	envParseTimeout         = "WORKOUT_EXPLORER_PARSE_TIMEOUT"
	envStorageLimit         = "WORKOUT_EXPLORER_STORAGE_LIMIT_MB"
	envBaseURL              = "WORKOUT_EXPLORER_BASE_URL"
	envTrustedProxy         = "WORKOUT_EXPLORER_TRUSTED_PROXY"
	envUploadsIPv4          = "WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4"
	envUploadsIPv6          = "WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV6"
	envPushoverToken        = "WORKOUT_EXPLORER_PUSHOVER_TOKEN"
	envPushoverUser         = "WORKOUT_EXPLORER_PUSHOVER_USER"
	envOperatorFile         = "WORKOUT_EXPLORER_OPERATOR_FILE"
)

const (
	defaultMaxConcurrentUploads = 4
	maxMaxConcurrentUploads     = 16
	minParseTimeoutS            = 5
	maxParseTimeoutS            = 60
	// The storage cap in decimal megabytes (1,000,000 bytes, as the 20 MB
	// upload cap, OQ-43): 1 GB by default, from 1 MB to 1 TB (F-10 design).
	defaultStorageLimitMB = 1_000
	maxStorageLimitMB     = 1_000_000
	bytesPerMB            = 1_000_000
	defaultUploadsPerHour = 60
	maxUploadsPerHour     = 240
)

var errMaxConcurrentUploads = errors.New(envMaxConcurrentUploads + " must be a whole number from 1 to 16")

var errParseTimeout = errors.New(envParseTimeout + " must be a whole number of seconds from 5 to 60")

var errStorageLimit = errors.New(envStorageLimit + " must be a whole number of megabytes from 1 to 1000000")

var errTrustedProxy = errors.New(envTrustedProxy + " must be one IP address")

var errUploadsIPv4 = errors.New(envUploadsIPv4 + " must be a whole number from 1 to 240")

var errUploadsIPv6 = errors.New(envUploadsIPv6 + " must be a whole number from 1 to 240")

var errBaseURL = errors.New(envBaseURL + " must be set to http://host[:port] or https://host[:port], with no path, trailing slash, user, query or fragment")

// hostPort: a DNS name (IDNs in punycode) or a bracketed IPv6 address, and an optional port.
var hostPort = regexp.MustCompile(`^([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*|\[[0-9A-Fa-f:.]+\])(:[0-9]*)?$`)

var wholeNumber = regexp.MustCompile(`^[1-9][0-9]*$`)

// boundedInt parses v as ASCII digits without sign, spaces or leading zero,
// at most as many as hi has, from lo to hi; ok is false otherwise.
func boundedInt(v string, lo, hi int) (n int, ok bool) {
	if !wholeNumber.MatchString(v) || len(v) > len(strconv.Itoa(hi)) {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil && n >= lo && n <= hi
}

// maxConcurrentUploads reads the number of uploads processed at once through
// getenv (os.Getenv in main, a map in tests): unset or empty is 4; otherwise
// a whole number from 1 to 16 (ADR-021 item 4).
func maxConcurrentUploads(getenv func(string) string) (int, error) {
	v := getenv(envMaxConcurrentUploads)
	if v == "" {
		return defaultMaxConcurrentUploads, nil
	}
	n, ok := boundedInt(v, 1, maxMaxConcurrentUploads)
	if !ok {
		return 0, errMaxConcurrentUploads
	}
	return n, nil
}

// parseTimeout reads the parser's time limit like maxConcurrentUploads: unset
// or empty is ingest.DefaultLimits.Timeout; otherwise whole seconds from 5 to
// 60 (ADR-017 rev. 7, F-09 D-30).
func parseTimeout(getenv func(string) string) (time.Duration, error) {
	v := getenv(envParseTimeout)
	if v == "" {
		return ingest.DefaultLimits.Timeout, nil
	}
	n, ok := boundedInt(v, minParseTimeoutS, maxParseTimeoutS)
	if !ok {
		return 0, errParseTimeout
	}
	return time.Duration(n) * time.Second, nil
}

// storageLimit reads the storage cap (UP-8) like maxConcurrentUploads: unset
// or empty is 1,000 MB; otherwise whole megabytes from 1 to 1,000,000. It
// returns bytes.
func storageLimit(getenv func(string) string) (int64, error) {
	v := getenv(envStorageLimit)
	if v == "" {
		return defaultStorageLimitMB * bytesPerMB, nil
	}
	n, ok := boundedInt(v, 1, maxStorageLimitMB)
	if !ok {
		return 0, errStorageLimit
	}
	return int64(n) * bytesPerMB, nil
}

// baseURL reads the origin of absolute links in Open Graph tags (F-23
// PC-8). Required, no default: unset or invalid is errBaseURL. It must
// re-serialise through net/url to the same string, which rules out upper-case
// schemes, a trailing slash and anything after the authority.
func baseURL(getenv func(string) string) (string, error) {
	v := getenv(envBaseURL)
	u, err := url.Parse(v)
	if v == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.Opaque != "" || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
		u.String() != v || !hostPort.MatchString(u.Host) {
		return "", errBaseURL
	}
	if p := u.Port(); p != "" || u.Host[len(u.Host)-1] == ':' {
		if _, ok := boundedInt(p, 1, 65535); !ok {
			return "", errBaseURL
		}
	}
	return v, nil
}

// uploadsPerHour reads one of the two upload limits (ADR-021 rev. 6 item 7):
// unset or empty is 60; otherwise a whole number from 1 to 240.
func uploadsPerHour(getenv func(string) string, name string, bad error) (int, error) {
	v := getenv(name)
	if v == "" {
		return defaultUploadsPerHour, nil
	}
	n, ok := boundedInt(v, 1, maxUploadsPerHour)
	if !ok {
		return 0, bad
	}
	return n, nil
}

// pushoverKeys reads the two notification secrets (ADR-039 item 1): both
// unset or empty is off ("", ""); one without the other, or a value that is
// not 30 ASCII letters and digits, is an error naming the variable, never
// its value.
func pushoverKeys(getenv func(string) string) (token, user string, err error) {
	token, user = getenv(envPushoverToken), getenv(envPushoverUser)
	switch {
	case token == "" && user == "":
		return "", "", nil
	case token == "":
		return "", "", errors.New(envPushoverToken + " must be set when " + envPushoverUser + " is set")
	case user == "":
		return "", "", errors.New(envPushoverUser + " must be set when " + envPushoverToken + " is set")
	case !pushover.ValidKey(token):
		return "", "", errors.New(envPushoverToken + " must be 30 letters and digits (a Pushover application token)")
	case !pushover.ValidKey(user):
		return "", "", errors.New(envPushoverUser + " must be 30 letters and digits (a Pushover user key)")
	}
	return token, user, nil
}

// operatorBlock reads the operator file (F-29 AB-8) and returns it rendered.
// Unset, empty or a file that does not exist is "" (the page shows its
// placeholder); a relative path, any other read error or invalid content is an
// error naming the setting.
func operatorBlock(getenv func(string) string) (string, error) {
	path := getenv(envOperatorFile)
	if path == "" {
		return "", nil
	}
	if !filepath.IsAbs(path) {
		return "", errors.New(envOperatorFile + " must be an absolute path")
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: cannot read the file: %v", envOperatorFile, err)
	}
	block, err := about.ParseOperator(b)
	if err != nil {
		return "", fmt.Errorf("%s: the file %v", envOperatorFile, err)
	}
	return block, nil
}

// trustedProxy reads Caddy's address (ADR-021 rev. 6 item 6): unset or empty
// is the zero Addr (loopback only); otherwise exactly one IP address, no zone,
// no prefix. IPv4-mapped IPv6 is unmapped to match the peer comparison.
func trustedProxy(getenv func(string) string) (netip.Addr, error) {
	v := getenv(envTrustedProxy)
	if v == "" {
		return netip.Addr{}, nil
	}
	a, err := netip.ParseAddr(v)
	if err != nil || a.Zone() != "" {
		return netip.Addr{}, errTrustedProxy
	}
	return a.Unmap(), nil
}

// readSettings reads every environment setting of serve in the order of the
// TD's table and returns the first error.
func readSettings(getenv func(string) string) (settings, error) {
	n, err := maxConcurrentUploads(getenv)
	if err != nil {
		return settings{}, err
	}
	t, err := parseTimeout(getenv)
	if err != nil {
		return settings{}, err
	}
	l, err := storageLimit(getenv)
	if err != nil {
		return settings{}, err
	}
	b, err := baseURL(getenv)
	if err != nil {
		return settings{}, err
	}
	tp, err := trustedProxy(getenv)
	if err != nil {
		return settings{}, err
	}
	u4, err := uploadsPerHour(getenv, envUploadsIPv4, errUploadsIPv4)
	if err != nil {
		return settings{}, err
	}
	u6, err := uploadsPerHour(getenv, envUploadsIPv6, errUploadsIPv6)
	if err != nil {
		return settings{}, err
	}
	pt, pu, err := pushoverKeys(getenv)
	if err != nil {
		return settings{}, err
	}
	op, err := operatorBlock(getenv)
	if err != nil {
		return settings{}, err
	}
	return settings{operatorHTML: op, operatorFile: getenv(envOperatorFile), maxConcurrentUploads: n, parseTimeout: t, storageLimit: l, baseURL: b, trustedProxy: tp, uploadsIPv4: u4, uploadsIPv6: u6,
		pushoverToken: pt, pushoverUser: pu}, nil
}

// newParser is the one place cmd builds an ingest.Parser: the default limits
// with only the time limit replaced. Zero t means the default. stored raises
// MaxBytes by the strip's growth, for regenerate's stored originals (F-30 RG-1).
func newParser(t time.Duration, stored bool, logger *slog.Logger) *ingest.Parser {
	lim := ingest.Limits{Timeout: t}
	if stored {
		lim.MaxBytes = ingest.DefaultLimits.MaxBytes + ingest.MaxStripGrowth
	}
	return ingest.NewParser(lim, logger)
}

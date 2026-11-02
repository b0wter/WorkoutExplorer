package httpapi

import (
	"hash/maphash"
	"net/http"
	"net/netip"
	"strings"
)

// clientPrefix is the identity the rate limit counts (ADR-021 item 6): the
// direct peer, or, when the peer is a trusted proxy (loopback, or exactly
// trusted: Caddy's address, F-31 DP-1a) and X-Forwarded-For is present, the
// last entry of that header, which is the client Caddy resolved. An unparsable
// entry falls back to the peer. Any other peer can never choose its address. IPv4-mapped IPv6 is unmapped;
// IPv4 counts per address, IPv6 per /64.
func clientPrefix(r *http.Request, trusted netip.Addr) netip.Prefix {
	var peer netip.Addr
	if ap, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		peer = ap.Addr().Unmap()
	}
	addr := peer
	if peer.IsLoopback() || (trusted.IsValid() && peer == trusted) {
		if vals := r.Header.Values("X-Forwarded-For"); len(vals) > 0 {
			last := vals[len(vals)-1]
			if i := strings.LastIndexByte(last, ','); i >= 0 {
				last = last[i+1:]
			}
			if a, err := netip.ParseAddr(strings.TrimSpace(last)); err == nil {
				addr = a.Unmap()
			}
		}
	}
	if !addr.IsValid() {
		return netip.Prefix{}
	}
	bits := 64
	if addr.Is4() {
		bits = 32
	}
	p, _ := addr.WithZone("").Prefix(bits)
	return p
}

// limiterKey is the table key of a prefix: a seeded 64-bit hash, process
// local, so the limiter never holds an address (ADR-021 item 8).
func limiterKey(seed maphash.Seed, p netip.Prefix) uint64 {
	b := p.Addr().As16()
	return maphash.Bytes(seed, b[:])
}

// Command protoserve serves the built page (build/web) and the prototype data
// files (build/prototype/data) with brotli, so a phone sees realistic transfer
// sizes during the phase 0 scrubbing gate (F-07, roadmap gate 0).
//
// It is for development only and is never deployed: production serving is
// F-31 (Caddy). It is not part of the workout-explorer binary.
//
// Design: architect/F-07-prototype-page-and-scrubbing-gate.md, section B-3.
//
//	protoserve -web <dir> -data <dir> [-addr :18080] [-tls]
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"workout-explorer/internal/precompress"
)

const usage = `usage: protoserve -web <dir> -data <dir> -samples <dir> [-addr :18080] [-tls]

Development only, never deployed (F-07). Serves <web dir> at / (except its
index.html, the page template), <data dir> at /data/ and each sample view model
of <samples dir> executed with the template at /w/<name> (F-13, B-1),
brotli-compressed when the request accepts it.

  -web   directory with the built page (contains index.html)
  -data  directory with the prototype data files
  -samples  directory with the sample view models (*.json)
  -addr  listen address (default :18080)
  -tls   serve HTTPS with an in-memory self-signed certificate
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses the flags, loads the files, prints the start-up block, listens
// and serves. It returns before listening on any error.
func run(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("protoserve", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	webDir := fl.String("web", "", "")
	dataDir := fl.String("data", "", "")
	samplesDir := fl.String("samples", "", "")
	addr := fl.String("addr", ":18080", "")
	useTLS := fl.Bool("tls", false, "")
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stderr, usage)
			return 0
		}
		fmt.Fprint(stderr, usage)
		return 2
	}
	if *webDir == "" || *dataDir == "" || *samplesDir == "" || *addr == "" || fl.NArg() > 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	files, err := load(*webDir, *dataDir)
	if err == nil {
		err = addSamples(files, *webDir, *samplesDir, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "protoserve: %v\n", err)
		return 1
	}
	ips := localIPs()
	var cert tls.Certificate
	if *useTLS {
		if cert, err = selfSigned(time.Now(), ips); err != nil {
			fmt.Fprintf(stderr, "protoserve: %v\n", err)
			return 1
		}
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(stderr, "protoserve: %v\n", err)
		return 1
	}
	defer ln.Close()

	fmt.Fprintln(stdout, "protoserve: development only, never deploy (F-07)")
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		f := files[k]
		br := "-"
		if f.br != nil {
			br = strconv.Itoa(len(f.br))
		}
		if f.gz > 0 {
			fmt.Fprintf(stdout, "%s  raw %d  gzip-9 %d  br %s\n", k, len(f.raw), f.gz, br)
			continue
		}
		fmt.Fprintf(stdout, "%s  raw %d  br %s\n", k, len(f.raw), br)
	}
	fmt.Fprintf(stdout, "protoserve: %d files\n", len(files))
	scheme := "http"
	if *useTLS {
		scheme = "https"
		fmt.Fprintf(stdout, "protoserve: TLS self-signed certificate, SHA-256 %x\n", sha256.Sum256(cert.Certificate[0]))
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			fmt.Fprintf(stdout, "url  %s://%s:%s/w/ride-12h?measure\n", scheme, ip4, port)
		}
	}
	fmt.Fprintf(stdout, "protoserve: listening on %s\n", ln.Addr())

	srv := &http.Server{
		Handler:           newHandler(files, stdout),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if *useTLS {
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		if *useTLS {
			errc <- srv.ServeTLS(ln, "", "")
		} else {
			errc <- srv.Serve(ln)
		}
	}()
	select {
	case err := <-errc:
		fmt.Fprintf(stderr, "protoserve: %v\n", err)
		return 1
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(sctx)
	fmt.Fprintln(stdout, "protoserve: stopped")
	return 0
}

// file is one served file: its bytes, the brotli encoding (nil when the type
// is not compressed) and its Content-Type.
type file struct {
	raw         []byte
	br          []byte
	gz          int // gzip -9 size, set for sample pages only
	contentType string
}

// load reads webDir and dataDir once. Keys are URL paths: "/<rel>" for the
// web folder and "/data/<rel>" for the data folder.
func load(webDir, dataDir string) (map[string]file, error) {
	if st, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil || !st.Mode().IsRegular() {
		return nil, fmt.Errorf("index.html not found in %s", webDir)
	}
	if st, err := os.Stat(dataDir); err != nil {
		return nil, fmt.Errorf("data directory %s: %w", dataDir, err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("data directory %s: not a directory", dataDir)
	}
	files := map[string]file{}
	for _, d := range []struct{ dir, prefix string }{{webDir, "/"}, {dataDir, "/data/"}} {
		err := filepath.WalkDir(d.dir, func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == d.dir {
				return nil
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				if e.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if e.IsDir() {
				return nil
			}
			if !e.Type().IsRegular() || strings.HasSuffix(strings.ToLower(name), ".br") {
				return nil
			}
			rel, err := filepath.Rel(d.dir, path)
			if err != nil {
				return err
			}
			key := d.prefix + filepath.ToSlash(rel)
			if _, dup := files[key]; dup {
				return fmt.Errorf("duplicate path %s", key)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			ctype, compress := contentType(name)
			f := file{raw: raw, contentType: ctype}
			if compress {
				if f.br, err = precompress.Brotli(raw); err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
			}
			files[key] = f
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// contentType returns the Content-Type for a file name and whether the file
// is served brotli-compressed.
func contentType(name string) (ctype string, compress bool) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8", true
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8", true
	case ".css":
		return "text/css; charset=utf-8", true
	case ".json":
		return "application/json", true
	case ".svg":
		return "image/svg+xml", true
	case ".txt":
		return "text/plain; charset=utf-8", true
	case ".bin":
		return "application/octet-stream", true
	case ".png":
		return "image/png", false
	case ".woff2":
		return "font/woff2", false
	}
	return "application/octet-stream", false
}

// acceptsBrotli reports whether an Accept-Encoding header accepts "br".
// "*" does not count; q=0 or an unparsable q refuses.
func acceptsBrotli(header string) bool {
	for _, el := range strings.Split(header, ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(el), ";")
		if strings.ToLower(strings.TrimSpace(coding)) != "br" {
			continue
		}
		for _, p := range strings.Split(params, ";") {
			name, val, ok := strings.Cut(p, "=")
			if !ok || strings.ToLower(strings.TrimSpace(name)) != "q" {
				continue
			}
			q, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
			if err != nil || q <= 0 {
				return false
			}
		}
		return true
	}
	return false
}

// newHandler serves the snapshot and logs one line per request to log.
func newHandler(files map[string]file, log io.Writer) http.Handler {
	var mu sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ae := acceptsBrotli(r.Header.Get("Accept-Encoding"))
		status, enc, n := serve(w, r, files, ae)
		aes := "no-br"
		if ae {
			aes = "br"
		}
		mu.Lock()
		fmt.Fprintf(log, "%s %s %d %s %d ae=%s\n", r.Method, r.URL.Path, status, enc, n, aes)
		mu.Unlock()
	})
}

// serve writes the response and returns the status, the encoding for the log
// and the number of body bytes written.
func serve(w http.ResponseWriter, r *http.Request, files map[string]file, ae bool) (status int, enc string, n int) {
	h := w.Header()
	plain := func(code int, body string) (int, string, int) {
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(code)
		if r.Method == http.MethodHead {
			return code, "-", 0
		}
		written, _ := io.WriteString(w, body)
		return code, "-", written
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.Set("Allow", "GET, HEAD")
		return plain(http.StatusMethodNotAllowed, "method not allowed\n")
	}
	f, ok := files[r.URL.Path]
	if !ok {
		return plain(http.StatusNotFound, "not found\n")
	}
	body, enc := f.raw, "identity"
	h.Set("Content-Type", f.contentType)
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	if f.br != nil {
		h.Set("Vary", "Accept-Encoding")
		if ae {
			body, enc = f.br, "br"
			h.Set("Content-Encoding", "br")
		}
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return http.StatusOK, enc, 0
	}
	written, _ := w.Write(body)
	return http.StatusOK, enc, written
}

// selfSigned creates an in-memory ECDSA P-256 certificate valid for localhost,
// the loopback addresses and ips.
func selfSigned(now time.Time, ips []net.IP) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certificate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certificate serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "protoserve (development only)"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(7 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{"localhost"},
		IPAddresses:           append([]net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}, ips...),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certificate: %w", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

// localIPs returns the addresses of the up, non-loopback interfaces.
func localIPs() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []net.IP
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				ips = append(ips, ipn.IP)
			}
		}
	}
	return ips
}

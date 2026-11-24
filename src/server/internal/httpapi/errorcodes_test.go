package httpapi

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"workout-explorer/internal/ingest"
)

// The committed list of API error codes is the contract with the browser
// (F-54 LG-6, ADR-059 item 5): testdata/api/error-codes.json.
type codeEntry struct {
	General  bool     `json:"general"`
	Messages []string `json:"messages"`
}

// msgCode maps each msg* constant of this package to the code it is sent with.
var msgCode = map[string]string{
	"msgImpressumRate": codeRateLimited, "msgReportDeleteRate": codeRateLimited, "msgReportsLimit": codeRateLimited,
	"msgReportType": codeInvalidRep, "msgReportBody": codeInvalidRep,
	"msgMapBusy": codeBusy, "msgWriteBusy": codeBusy, "msgMapStale": codeStale, "msgMapRefused": codeInvalidImage + "|" + codeTooLarge,
	"msgBadBody": codeInvalid, "msgBadTrim": codeInvalid, "msgBadZone": codeInvalid, "msgBadType": codeInvalid,
}

// stringConsts returns the string constants of the non-test files of dir.
func stringConsts(t *testing.T, dir string) map[string]string {
	t.Helper()
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, p := range pkgs {
		for _, f := range p.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				vs, ok := n.(*ast.ValueSpec)
				if !ok {
					return true
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						out[name.Name], _ = strconv.Unquote(lit.Value)
					}
				}
				return true
			})
		}
	}
	return out
}

func TestErrorCodeList(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/api/error-codes.json")
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Codes map[string]codeEntry `json:"codes"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}

	// What the server can send, from the source: code constants, msg
	// constants, the refusal table, the two fixed bodies and ingest's kinds.
	sent := map[string]map[string]bool{}
	add := func(code, msg string) {
		if sent[code] == nil {
			sent[code] = map[string]bool{}
		}
		sent[code][msg] = true
	}
	consts := stringConsts(t, ".")
	for name, v := range consts {
		if strings.HasPrefix(name, "code") {
			add(v, "")
		}
	}
	for name, v := range consts {
		if !strings.HasPrefix(name, "msg") {
			continue
		}
		codes, ok := msgCode[name]
		if !ok {
			t.Errorf("%s is a message constant without a code: add it to msgCode", name)
		}
		for _, c := range strings.Split(codes, "|") {
			add(c, v)
		}
	}
	for c, r := range refusals {
		add(c, r.message)
	}
	for _, b := range [][]byte{notFoundBody, linkInvalidBody} {
		var e errorBody
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		add(e.Error, e.Message)
	}
	for k := range stringConsts(t, "../ingest") {
		if !strings.HasPrefix(k, "Kind") {
			continue
		}
		kind := ingest.Kind(stringConsts(t, "../ingest")[k])
		add(string(kind), (&ingest.Error{Kind: kind}).Message())
	}

	for code, msgs := range sent {
		e, ok := list.Codes[code]
		if !ok {
			t.Errorf("code %q can be sent but is not in error-codes.json", code)
			continue
		}
		if e.General {
			continue
		}
		var got []string
		for m := range msgs {
			if m != "" {
				got = append(got, m)
			}
		}
		sort.Strings(got)
		want := append([]string(nil), e.Messages...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("code %q: server sentences %q, list %q", code, got, want)
		}
	}
	for code := range list.Codes {
		if _, ok := sent[code]; !ok {
			t.Errorf("code %q is in error-codes.json but the server never sends it", code)
		}
	}
}

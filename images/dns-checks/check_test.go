package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestNormalize(t *testing.T) {
	got := normalize("NS-2.example.net., ns-1.example.net, NS-2.example.net.")
	want := []string{"ns-1.example.net", "ns-2.example.net"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestContainsAll(t *testing.T) {
	obs := []string{"ns-1.example.net", "ns-2.example.net", "ns-3.example.net"}
	if !containsAll(obs, []string{"ns-2.example.net", "ns-1.example.net"}) {
		t.Fatal("expected subset to match")
	}
	if containsAll(obs, []string{"ns-9.example.net"}) {
		t.Fatal("unexpected match")
	}
}

func TestCheckSkipped(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	var stdout bytes.Buffer
	check(&stdout, config{output: outPath, attempts: 1}, client{})
	logs := jsonLogMsgs(t, stdout.Bytes())
	if !contains(logs, "skipped") || !contains(logs, "wrote outputs") {
		t.Fatalf("logs %v", logs)
	}
	if bytes.Contains(stdout.Bytes(), []byte("====")) {
		t.Fatalf("banner still present: %s", stdout.Bytes())
	}
	var got result
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "SKIPPED" || got.Delegated != "unknown" {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckDelegatedWritesJSONAndLogs(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	var stdout bytes.Buffer
	c := client{
		trace: func(string) (nsTrace, error) {
			return nsTrace{
				allNS:    []string{"a.root-servers.net", "ns-1.example.net"},
				observed: []string{"ns-1.example.net", "ns-2.example.net"},
			}, nil
		},
	}
	check(&stdout, config{
		domain:      "install.example.com",
		nameservers: "NS-2.example.net.,ns-1.example.net",
		output:      outPath,
		attempts:    1,
	}, c)
	logs := jsonLogMsgs(t, stdout.Bytes())
	if !contains(logs, "checking dns") || !contains(logs, "attempt complete") || !contains(logs, "wrote outputs") {
		t.Fatalf("logs %v stdout %s", logs, stdout.Bytes())
	}
	if bytes.Contains(stdout.Bytes(), []byte("-> OK:")) {
		t.Fatalf("plaintext log still present: %s", stdout.Bytes())
	}
	var got result
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "DELEGATED" || got.Delegated != "true" {
		t.Fatalf("got %+v", got)
	}
	if got.ObservedNameservers != "ns-1.example.net,ns-2.example.net" {
		t.Fatalf("observed %q", got.ObservedNameservers)
	}
	if got.UpdatedAt == "" {
		t.Fatal("missing updated_at")
	}
}

func TestCheckNotDelegatedRetriesThenWrites(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	var stdout bytes.Buffer
	calls := 0
	c := client{
		trace: func(string) (nsTrace, error) {
			calls++
			return nsTrace{allNS: []string{"a.root-servers.net", "a.gtld-servers.net"}}, nil
		},
	}
	check(&stdout, config{
		domain:      "install.example.com",
		nameservers: "ns-1.example.net",
		output:      outPath,
		attempts:    2,
		sleep:       time.Millisecond,
	}, c)
	if calls != 2 {
		t.Fatalf("calls %d", calls)
	}
	var got result
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "NOT_DELEGATED" || got.Delegated != "false" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveFlagFallsBackToEnv(t *testing.T) {
	t.Setenv("DOMAIN", "from-env.example")
	if got := resolve("$DOMAIN", "DOMAIN", ""); got != "from-env.example" {
		t.Fatalf("got %q", got)
	}
	if got := resolve("explicit.example", "DOMAIN", ""); got != "explicit.example" {
		t.Fatalf("got %q", got)
	}
}

func TestParseChecks(t *testing.T) {
	if strings.Join(parseChecks(""), ",") != "delegation" {
		t.Fatal(parseChecks(""))
	}
	if strings.Join(parseChecks("caa, ds,SOA"), ",") != "soa,caa,ds" {
		t.Fatal(parseChecks("caa, ds,SOA"))
	}
}

func TestCheckStrictNameservers(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	var stdout bytes.Buffer
	c := client{
		trace: func(string) (nsTrace, error) {
			return nsTrace{
				allNS:    []string{"a.root-servers.net"},
				observed: []string{"ns-1.example.net", "ns-2.example.net", "stale.example.net"},
			}, nil
		},
	}
	check(&stdout, config{
		domain:      "install.example.com",
		nameservers: "ns-1.example.net,ns-2.example.net",
		output:      outPath,
		attempts:    1,
		strictNS:    true,
	}, c)
	got := readResult(t, outPath)
	if got.Status != "DELEGATED_ELSEWHERE" {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckSOAAndCAAAndDS(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	var stdout bytes.Buffer
	c := client{
		trace: func(string) (nsTrace, error) {
			return nsTrace{
				allNS:    []string{"a.root-servers.net", "ns-1.example.net"},
				observed: []string{"ns-1.example.net", "ns-2.example.net"},
				parentIP: "192.5.6.30",
			}, nil
		},
		lookup: func(hosts []string) []string {
			return []string{"1.1.1.1"}
		},
		query: func(qname string, qtype uint16, servers []string) *dns.Msg {
			msg := new(dns.Msg)
			switch qtype {
			case dns.TypeSOA:
				msg.Answer = append(msg.Answer, soa(qname+"."))
			case dns.TypeCAA:
				msg.Answer = append(msg.Answer, &dns.CAA{
					Hdr:   dns.RR_Header{Name: qname + ".", Rrtype: dns.TypeCAA, Class: dns.ClassINET, Ttl: 60},
					Tag:   "issue",
					Value: "letsencrypt.org",
				})
			case dns.TypeDS:
				return msg
			}
			return msg
		},
	}
	check(&stdout, config{
		domain:      "install.example.com",
		nameservers: "ns-1.example.net,ns-2.example.net",
		output:      outPath,
		attempts:    1,
		checks:      []string{checkDelegation, checkSOA, checkCAA, checkDS},
		caaIssuers:  []string{"letsencrypt.org"},
	}, c)
	got := readResult(t, outPath)
	if got.Status != "OK" || got.DelegationStatus != "DELEGATED" || got.SOAStatus != "OK" || got.CAAStatus != "OK" || got.DSStatus != "NONE" {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckCAABlockedAndDSPresent(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	var stdout bytes.Buffer
	c := client{
		trace: func(string) (nsTrace, error) {
			return nsTrace{
				allNS:    []string{"a.root-servers.net"},
				observed: []string{"ns-1.example.net"},
				parentIP: "192.5.6.30",
			}, nil
		},
		lookup: func([]string) []string { return []string{"1.1.1.1"} },
		query: func(qname string, qtype uint16, _ []string) *dns.Msg {
			msg := new(dns.Msg)
			switch qtype {
			case dns.TypeCAA:
				msg.Answer = append(msg.Answer, &dns.CAA{
					Hdr:   dns.RR_Header{Name: qname + ".", Rrtype: dns.TypeCAA, Class: dns.ClassINET, Ttl: 60},
					Tag:   "issue",
					Value: "pki.goog",
				})
			case dns.TypeDS:
				msg.Ns = append(msg.Ns, &dns.DS{
					Hdr:        dns.RR_Header{Name: qname + ".", Rrtype: dns.TypeDS, Class: dns.ClassINET, Ttl: 60},
					KeyTag:     12345,
					Algorithm:  8,
					DigestType: 2,
					Digest:     "abcd",
				})
			}
			return msg
		},
	}
	check(&stdout, config{
		domain:      "install.example.com",
		nameservers: "ns-1.example.net",
		output:      outPath,
		attempts:    1,
		checks:      []string{checkDelegation, checkCAA, checkDS},
		caaIssuers:  []string{"letsencrypt.org"},
	}, c)
	got := readResult(t, outPath)
	if got.Status != "FAILED" || got.CAAStatus != "BLOCKED" || got.DSStatus != "PRESENT" || got.DelegationStatus != "DELEGATED" {
		t.Fatalf("got %+v", got)
	}
}

func readResult(t *testing.T, path string) result {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got result
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func jsonLogMsgs(t *testing.T, raw []byte) []string {
	t.Helper()
	var msgs []string
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var rec struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("log line not json: %s (%v)", line, err)
		}
		msgs = append(msgs, rec.Msg)
	}
	return msgs
}

func contains(got []string, want string) bool {
	for _, g := range got {
		if g == want {
			return true
		}
	}
	return false
}

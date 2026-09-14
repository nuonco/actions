package main

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestTraceNSFollowsDelegation(t *testing.T) {
	exch := fakeTree(map[string][]dns.RR{
		"198.41.0.4|example.com.|NS": {
			ns("com.", "a.gtld-servers.net."),
			a("a.gtld-servers.net.", "192.5.6.30"),
		},
		"192.5.6.30|example.com.|NS": {
			ns("example.com.", "ns-1.awsdns-00.com."),
			ns("example.com.", "ns-2.awsdns-00.net."),
			a("ns-1.awsdns-00.com.", "1.1.1.1"),
		},
	})
	got, err := traceNS("example.com", exch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.allNS) == 0 {
		t.Fatal("expected NS from the walk")
	}
	if strings.Join(got.observed, ",") != "ns-1.awsdns-00.com,ns-2.awsdns-00.net" {
		t.Fatalf("observed %v", got.observed)
	}
	if got.parentIP != "192.5.6.30" {
		t.Fatalf("parent %q", got.parentIP)
	}
}

func TestTraceNSUndelegated(t *testing.T) {
	exch := fakeTree(map[string][]dns.RR{
		"198.41.0.4|missing.example.com.|NS": {
			ns("com.", "a.gtld-servers.net."),
			a("a.gtld-servers.net.", "192.5.6.30"),
		},
		"192.5.6.30|missing.example.com.|NS": {
			soa("example.com."),
		},
	})
	got, err := traceNS("missing.example.com", exch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.observed) != 0 {
		t.Fatalf("observed %v", got.observed)
	}
	if len(got.allNS) == 0 {
		t.Fatal("expected parent NS in the walk")
	}
	if got.parentIP != "192.5.6.30" {
		t.Fatalf("parent %q", got.parentIP)
	}
}

func fakeTree(answers map[string][]dns.RR) exchanger {
	return func(req *dns.Msg, addr string) (*dns.Msg, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if len(req.Question) == 0 {
			return nil, fmt.Errorf("no question")
		}
		q := req.Question[0]
		key := host + "|" + strings.ToLower(q.Name) + "|" + dns.TypeToString[q.Qtype]
		rrs, ok := answers[key]
		if !ok {
			return nil, fmt.Errorf("unexpected query %s", key)
		}
		msg := new(dns.Msg)
		msg.SetReply(req)
		msg.Authoritative = false
		for _, rr := range rrs {
			switch rr.(type) {
			case *dns.A, *dns.AAAA:
				msg.Extra = append(msg.Extra, rr)
			default:
				msg.Ns = append(msg.Ns, rr)
			}
		}
		return msg, nil
	}
}

func ns(owner, target string) dns.RR {
	return &dns.NS{Hdr: dns.RR_Header{Name: owner, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60}, Ns: target}
}

func a(owner, ip string) dns.RR {
	return &dns.A{Hdr: dns.RR_Header{Name: owner, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(ip).To4()}
}

func soa(owner string) dns.RR {
	return &dns.SOA{
		Hdr:     dns.RR_Header{Name: owner, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 60},
		Ns:      "a.gtld-servers.net.",
		Mbox:    "nstld.verisign-grs.com.",
		Serial:  1,
		Refresh: 1800,
		Retry:   900,
		Expire:  604800,
		Minttl:  86400,
	}
}

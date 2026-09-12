package main

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var rootHints = []string{
	"198.41.0.4", "199.9.14.201", "192.33.4.12", "199.7.91.13",
	"192.203.230.10", "192.5.5.241", "192.112.36.4", "198.97.190.53",
	"192.36.148.17", "192.58.128.30", "193.0.14.129", "199.7.83.42",
	"202.12.27.33",
}

type exchanger func(req *dns.Msg, addr string) (*dns.Msg, error)

type nsTrace struct {
	allNS    []string
	observed []string
	parentIP string
}

func liveClient() client {
	exch := defaultExchange
	return client{
		trace: func(domain string) (nsTrace, error) {
			return traceNS(domain, exch)
		},
		query: func(qname string, qtype uint16, servers []string) *dns.Msg {
			resp, _ := queryFirst(exch, dns.Fqdn(qname), qtype, servers)
			return resp
		},
		lookup: func(hosts []string) []string {
			return lookupNSAddrs(hosts, "", exch)
		},
	}
}

func defaultExchange(req *dns.Msg, addr string) (*dns.Msg, error) {
	c := &dns.Client{Timeout: 3 * time.Second, Net: "udp"}
	resp, _, err := c.Exchange(req, addr)
	if err != nil || resp == nil || resp.Truncated {
		c.Net = "tcp"
		resp, _, err = c.Exchange(req, addr)
	}
	return resp, err
}

func traceNS(domain string, exch exchanger) (nsTrace, error) {
	qname := dns.Fqdn(domain)
	servers := append([]string(nil), rootHints...)
	seenNS := map[string]struct{}{}
	var lastErr error
	var parentIP string

	for hop := 0; hop < 20; hop++ {
		resp, from := queryFirst(exch, qname, dns.TypeNS, servers)
		if resp == nil {
			if lastErr == nil {
				lastErr = fmt.Errorf("no nameserver responded for %s", qname)
			}
			break
		}
		parentIP = from

		collectNS(resp, seenNS)
		if got := nsFor(resp, qname); len(got) > 0 {
			return nsTrace{allNS: sortedKeys(seenNS), observed: got, parentIP: from}, nil
		}

		next, glue := referral(resp, qname)
		if len(next) == 0 {
			break
		}
		servers = glue
		if len(servers) == 0 {
			servers = lookupNSAddrs(next, from, exch)
		}
		if len(servers) == 0 {
			lastErr = fmt.Errorf("referral for %s had no reachable nameservers", qname)
			break
		}
	}

	keys := sortedKeys(seenNS)
	if len(keys) == 0 {
		if lastErr != nil {
			return nsTrace{}, lastErr
		}
		return nsTrace{}, fmt.Errorf("empty trace")
	}
	return nsTrace{allNS: keys, parentIP: parentIP}, nil
}

func queryFirst(exch exchanger, qname string, qtype uint16, servers []string) (*dns.Msg, string) {
	req := new(dns.Msg)
	req.SetQuestion(qname, qtype)
	req.RecursionDesired = false
	for try := 0; try < 2; try++ {
		for _, ip := range servers {
			addr := net.JoinHostPort(ip, "53")
			resp, err := exch(req, addr)
			if err != nil || resp == nil {
				continue
			}
			return resp, ip
		}
	}
	return nil, ""
}

func collectNS(msg *dns.Msg, seen map[string]struct{}) {
	for _, rr := range append(append(msg.Answer, msg.Ns...), msg.Extra...) {
		ns, ok := rr.(*dns.NS)
		if !ok {
			continue
		}
		host := strings.ToLower(strings.TrimSuffix(ns.Ns, "."))
		if host != "" {
			seen[host] = struct{}{}
		}
	}
}

func nsFor(msg *dns.Msg, qname string) []string {
	want := strings.ToLower(qname)
	seen := map[string]struct{}{}
	for _, rr := range append(msg.Answer, msg.Ns...) {
		ns, ok := rr.(*dns.NS)
		if !ok {
			continue
		}
		if strings.ToLower(ns.Hdr.Name) != want {
			continue
		}
		host := strings.ToLower(strings.TrimSuffix(ns.Ns, "."))
		if host != "" {
			seen[host] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func referral(msg *dns.Msg, qname string) (hosts, addrs []string) {
	q := strings.ToLower(qname)
	hostSet := map[string]struct{}{}
	for _, rr := range msg.Ns {
		ns, ok := rr.(*dns.NS)
		if !ok {
			continue
		}
		owner := strings.ToLower(ns.Hdr.Name)
		if owner == q || !dns.IsSubDomain(owner, q) {
			continue
		}
		hostSet[strings.ToLower(strings.TrimSuffix(ns.Ns, "."))] = struct{}{}
	}
	hosts = sortedKeys(hostSet)
	addrSet := map[string]struct{}{}
	for _, rr := range msg.Extra {
		name := strings.ToLower(strings.TrimSuffix(rr.Header().Name, "."))
		if _, ok := hostSet[name]; !ok {
			continue
		}
		switch a := rr.(type) {
		case *dns.A:
			addrSet[a.A.String()] = struct{}{}
		case *dns.AAAA:
			addrSet[a.AAAA.String()] = struct{}{}
		}
	}
	return hosts, sortedKeys(addrSet)
}

func lookupNSAddrs(hosts []string, hint string, exch exchanger) []string {
	addrSet := map[string]struct{}{}
	for _, host := range hosts {
		req := new(dns.Msg)
		req.SetQuestion(dns.Fqdn(host), dns.TypeA)
		req.RecursionDesired = false
		if hint != "" {
			if resp, err := exch(req, net.JoinHostPort(hint, "53")); err == nil {
				for _, rr := range resp.Answer {
					if a, ok := rr.(*dns.A); ok {
						addrSet[a.A.String()] = struct{}{}
					}
				}
			}
		}
		if ips, err := net.LookupIP(host); err == nil {
			for _, ip := range ips {
				addrSet[ip.String()] = struct{}{}
			}
		}
	}
	return sortedKeys(addrSet)
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

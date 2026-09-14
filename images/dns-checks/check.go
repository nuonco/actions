package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const (
	checkDelegation = "delegation"
	checkSOA        = "soa"
	checkCAA        = "caa"
	checkDS         = "ds"
)

var knownChecks = []string{checkDelegation, checkSOA, checkCAA, checkDS}

type config struct {
	domain      string
	nameservers string
	output      string
	attempts    int
	sleep       time.Duration
	checks      []string
	strictNS    bool
	caaIssuers  []string
}

type result struct {
	Status              string `json:"status"`
	Checks              string `json:"checks,omitempty"`
	Domain              string `json:"domain"`
	ExpectedNameservers string `json:"expected_nameservers,omitempty"`
	ObservedNameservers string `json:"observed_nameservers,omitempty"`
	Delegated           string `json:"delegated,omitempty"`
	DelegationStatus    string `json:"delegation_status,omitempty"`
	SOAStatus           string `json:"soa_status,omitempty"`
	SOAResponders       string `json:"soa_responders,omitempty"`
	CAAStatus           string `json:"caa_status,omitempty"`
	CAARecords          string `json:"caa_records,omitempty"`
	DSStatus            string `json:"ds_status,omitempty"`
	DSRecords           string `json:"ds_records,omitempty"`
	Diagnosis           string `json:"diagnosis"`
	UpdatedAt           string `json:"updated_at"`
}

type client struct {
	trace  func(domain string) (nsTrace, error)
	query  func(qname string, qtype uint16, servers []string) *dns.Msg
	lookup func(hosts []string) []string
}

type checkOutcome struct {
	name   string
	status string
	ok     bool
	skip   bool
	diag   string
}

func (c config) selected() []string {
	if len(c.checks) == 0 {
		return []string{checkDelegation}
	}
	return c.checks
}

func (c config) wants(name string) bool {
	for _, s := range c.selected() {
		if s == name {
			return true
		}
	}
	return false
}

func check(stdout io.Writer, cfg config, c client) {
	log := slog.New(slog.NewJSONHandler(stdout, nil))
	selected := cfg.selected()
	if len(cfg.caaIssuers) == 0 {
		cfg.caaIssuers = []string{"letsencrypt.org"}
	}

	out := result{
		Status:              "UNKNOWN",
		Checks:              strings.Join(selected, ","),
		Domain:              cfg.domain,
		ExpectedNameservers: cfg.nameservers,
	}

	needNS := cfg.wants(checkDelegation) || cfg.wants(checkSOA) || cfg.wants(checkCAA)
	switch {
	case cfg.domain == "":
		out.Status = "SKIPPED"
		if cfg.wants(checkDelegation) {
			out.Delegated = "unknown"
		}
		out.Diagnosis = "DOMAIN is empty — the sandbox DNS outputs are unpopulated, nothing to check."
		if needNS && cfg.nameservers == "" {
			out.Diagnosis = "DOMAIN or NAMESERVERS is empty — the sandbox DNS outputs are unpopulated, nothing to check."
		}
		log.Info("skipped", attrs(out)...)
		emit(log, cfg.output, out)
		return
	case needNS && cfg.nameservers == "" && !cfg.wants(checkDS):
		out.Status = "SKIPPED"
		if cfg.wants(checkDelegation) {
			out.Delegated = "unknown"
		}
		out.Diagnosis = "DOMAIN or NAMESERVERS is empty — the sandbox DNS outputs are unpopulated, nothing to check."
		log.Info("skipped", attrs(out)...)
		emit(log, cfg.output, out)
		return
	}

	expected := normalize(cfg.nameservers)
	log.Info("checking dns",
		"domain", cfg.domain,
		"checks", selected,
		"expected_nameservers", expected,
		"attempts", cfg.attempts,
	)

	for attempt := 1; attempt <= cfg.attempts; attempt++ {
		log.Info("attempt", "attempt", attempt, "attempts", cfg.attempts)
		runAttempt(log, cfg, c, expected, &out)
		outcomes := outcomesOf(cfg, out)
		out.Status, out.Diagnosis = rollup(selected, outcomes)
		log.Info("attempt complete", append(attrs(out), slog.Int("attempt", attempt))...)
		if passed(selected, outcomes) {
			emit(log, cfg.output, out)
			return
		}
		if attempt < cfg.attempts {
			time.Sleep(cfg.sleep)
		}
	}

	emit(log, cfg.output, out)
}

func runAttempt(log *slog.Logger, cfg config, c client, expected []string, out *result) {
	var tr nsTrace
	var traced bool
	ensureTrace := func() nsTrace {
		if traced {
			return tr
		}
		traced = true
		got, err := c.trace(cfg.domain)
		if err != nil {
			log.Info("trace failed", "error", err.Error())
			tr = nsTrace{}
			return tr
		}
		tr = got
		return tr
	}

	if cfg.wants(checkDelegation) {
		runDelegation(cfg, expected, out, ensureTrace())
	}
	if cfg.wants(checkDS) {
		runDS(cfg, c, out, ensureTrace())
	}
	if cfg.wants(checkSOA) {
		runSOA(cfg, c, expected, out)
	}
	if cfg.wants(checkCAA) {
		runCAA(cfg, c, expected, out)
	}
}

func runDelegation(cfg config, expected []string, out *result, tr nsTrace) {
	out.ObservedNameservers = strings.Join(tr.observed, ",")
	switch {
	case len(tr.allNS) == 0:
		out.DelegationStatus = "DNS_UNREACHABLE"
		out.Delegated = "unknown"
	case len(tr.observed) == 0:
		out.DelegationStatus = "NOT_DELEGATED"
		out.Delegated = "false"
	case nsMatch(tr.observed, expected, cfg.strictNS):
		out.DelegationStatus = "DELEGATED"
		out.Delegated = "true"
	default:
		out.DelegationStatus = "DELEGATED_ELSEWHERE"
		out.Delegated = "false"
	}
}

func runDS(cfg config, c client, out *result, tr nsTrace) {
	if tr.parentIP == "" {
		if len(tr.allNS) == 0 {
			out.DSStatus = "DNS_UNREACHABLE"
			return
		}
		out.DSStatus = "SKIPPED"
		return
	}
	msg := c.query(cfg.domain, dns.TypeDS, []string{tr.parentIP})
	records := dsRecords(msg)
	out.DSRecords = strings.Join(records, ",")
	if len(records) == 0 {
		out.DSStatus = "NONE"
		return
	}
	out.DSStatus = "PRESENT"
}

func runSOA(cfg config, c client, expected []string, out *result) {
	if len(expected) == 0 {
		out.SOAStatus = "SKIPPED"
		return
	}
	var responded []string
	for _, host := range expected {
		if answersType(c, cfg.domain, dns.TypeSOA, host) {
			responded = append(responded, host)
		}
	}
	out.SOAResponders = strings.Join(responded, ",")
	if len(responded) == len(expected) {
		out.SOAStatus = "OK"
		return
	}
	out.SOAStatus = "FAILED"
}

func runCAA(cfg config, c client, expected []string, out *result) {
	if len(expected) == 0 {
		out.CAAStatus = "SKIPPED"
		return
	}
	var records []string
	seen := map[string]struct{}{}
	for _, host := range expected {
		for _, rec := range caaAt(c, cfg.domain, host) {
			if _, ok := seen[rec]; ok {
				continue
			}
			seen[rec] = struct{}{}
			records = append(records, rec)
		}
	}
	sort.Strings(records)
	out.CAARecords = strings.Join(records, ",")
	switch {
	case len(records) == 0:
		out.CAAStatus = "EMPTY"
	case caaAllows(records, cfg.caaIssuers):
		out.CAAStatus = "OK"
	default:
		out.CAAStatus = "BLOCKED"
	}
}

func answersType(c client, qname string, qtype uint16, host string) bool {
	for _, ip := range c.lookup([]string{host}) {
		msg := c.query(qname, qtype, []string{ip})
		if msg == nil {
			continue
		}
		for _, rr := range allRR(msg) {
			if rr.Header().Rrtype == qtype && strings.EqualFold(strings.TrimSuffix(rr.Header().Name, "."), strings.TrimSuffix(qname, ".")) {
				return true
			}
		}
	}
	return false
}

func caaAt(c client, qname, host string) []string {
	var out []string
	for _, ip := range c.lookup([]string{host}) {
		msg := c.query(qname, dns.TypeCAA, []string{ip})
		out = append(out, caaRecords(msg)...)
	}
	return out
}

func allRR(msg *dns.Msg) []dns.RR {
	if msg == nil {
		return nil
	}
	return append(append(append([]dns.RR{}, msg.Answer...), msg.Ns...), msg.Extra...)
}

func caaRecords(msg *dns.Msg) []string {
	var out []string
	for _, rr := range allRR(msg) {
		caa, ok := rr.(*dns.CAA)
		if !ok || !strings.EqualFold(caa.Tag, "issue") {
			continue
		}
		out = append(out, strings.ToLower(strings.TrimSpace(strings.Split(caa.Value, ";")[0])))
	}
	return out
}

func dsRecords(msg *dns.Msg) []string {
	var out []string
	for _, rr := range allRR(msg) {
		ds, ok := rr.(*dns.DS)
		if !ok {
			continue
		}
		out = append(out, fmt.Sprintf("%d-%d-%d", ds.KeyTag, ds.Algorithm, ds.DigestType))
	}
	sort.Strings(out)
	return out
}

func caaAllows(records, issuers []string) bool {
	set := map[string]struct{}{}
	for _, r := range records {
		set[r] = struct{}{}
	}
	for _, issuer := range issuers {
		if _, ok := set[strings.ToLower(issuer)]; !ok {
			return false
		}
	}
	return len(issuers) > 0
}

func outcomesOf(cfg config, out result) []checkOutcome {
	var o []checkOutcome
	if cfg.wants(checkDelegation) {
		o = append(o, delegationOutcome(cfg, out))
	}
	if cfg.wants(checkSOA) {
		o = append(o, soaOutcome(cfg, out))
	}
	if cfg.wants(checkCAA) {
		o = append(o, caaOutcome(cfg, out))
	}
	if cfg.wants(checkDS) {
		o = append(o, dsOutcome(cfg, out))
	}
	return o
}

func delegationOutcome(cfg config, out result) checkOutcome {
	c := checkOutcome{name: checkDelegation, status: out.DelegationStatus}
	switch out.DelegationStatus {
	case "DELEGATED":
		c.ok = true
		c.diag = fmt.Sprintf("The parent zone delegates %s to our nameservers.", cfg.domain)
	case "NOT_DELEGATED":
		c.diag = fmt.Sprintf("No parent zone holds NS records for %s. Create the delegation in the parent zone, pointing at: %s", cfg.domain, cfg.nameservers)
	case "DELEGATED_ELSEWHERE":
		c.diag = fmt.Sprintf("%s is delegated to a different nameserver set (%s); expected %s. Usually a stale delegation left over from a previous install on the same domain.", cfg.domain, out.ObservedNameservers, cfg.nameservers)
	case "DNS_UNREACHABLE":
		c.diag = "The trace returned no NS records at all, not even for the root. DNS is unreachable from the runner — this is not the same as the zone being undelegated."
	default:
		c.skip = true
	}
	return c
}

func soaOutcome(cfg config, out result) checkOutcome {
	c := checkOutcome{name: checkSOA, status: out.SOAStatus}
	switch out.SOAStatus {
	case "OK":
		c.ok = true
		c.diag = fmt.Sprintf("All expected nameservers answer SOA for %s.", cfg.domain)
	case "FAILED":
		c.diag = fmt.Sprintf("Expected nameservers did not all answer SOA for %s (responded: %s).", cfg.domain, out.SOAResponders)
	default:
		c.skip = true
	}
	return c
}

func caaOutcome(cfg config, out result) checkOutcome {
	c := checkOutcome{name: checkCAA, status: out.CAAStatus}
	issuers := strings.Join(cfg.caaIssuers, ",")
	switch out.CAAStatus {
	case "OK":
		c.ok = true
		c.diag = fmt.Sprintf("CAA at %s allows %s.", cfg.domain, issuers)
	case "BLOCKED":
		c.diag = fmt.Sprintf("CAA at %s does not allow %s (records: %s). Let's Encrypt will refuse issuance.", cfg.domain, issuers, out.CAARecords)
	case "EMPTY":
		c.diag = fmt.Sprintf("No CAA records at %s on the expected nameservers. The sandbox zone should publish issue letsencrypt.org.", cfg.domain)
	default:
		c.skip = true
	}
	return c
}

func dsOutcome(cfg config, out result) checkOutcome {
	c := checkOutcome{name: checkDS, status: out.DSStatus}
	switch out.DSStatus {
	case "NONE":
		c.ok = true
		c.diag = fmt.Sprintf("No DS records at the parent for %s.", cfg.domain)
	case "PRESENT":
		c.diag = fmt.Sprintf("Parent has DS records for %s (%s). The Route 53 zone is unsigned, so validating resolvers will treat the domain as bogus.", cfg.domain, out.DSRecords)
	case "DNS_UNREACHABLE":
		c.diag = "The trace returned no NS records at all, not even for the root. DNS is unreachable from the runner — this is not the same as the zone being undelegated."
	default:
		c.skip = true
	}
	return c
}

func rollup(selected []string, outcomes []checkOutcome) (status, diagnosis string) {
	var diags []string
	ran := 0
	ok := 0
	unreachable := false
	for _, o := range outcomes {
		if o.skip {
			continue
		}
		ran++
		if o.status == "DNS_UNREACHABLE" {
			unreachable = true
		}
		if o.ok {
			ok++
		}
		if o.diag != "" && (!o.ok || len(selected) == 1) {
			diags = append(diags, o.diag)
		}
	}
	if ran == 0 {
		return "SKIPPED", "No selected checks could run."
	}
	if unreachable && ok == 0 {
		return "DNS_UNREACHABLE", strings.Join(diags, " ")
	}
	if len(selected) == 1 {
		return outcomes[0].status, strings.Join(diags, " ")
	}
	if ok == ran {
		var okDiags []string
		for _, o := range outcomes {
			if o.ok {
				okDiags = append(okDiags, o.diag)
			}
		}
		return "OK", strings.Join(okDiags, " ")
	}
	var failDiags []string
	for _, o := range outcomes {
		if !o.skip && !o.ok {
			failDiags = append(failDiags, o.diag)
		}
	}
	return "FAILED", strings.Join(failDiags, " ")
}

func passed(selected []string, outcomes []checkOutcome) bool {
	status, _ := rollup(selected, outcomes)
	switch status {
	case "DELEGATED", "OK", "NONE":
		return true
	default:
		return false
	}
}

func emit(log *slog.Logger, path string, out result) {
	out.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	payload, err := json.Marshal(out)
	if err != nil {
		log.Error("failed to encode outputs", "error", err.Error())
		return
	}
	if err := appendFile(path, append(payload, '\n')); err != nil {
		log.Error("failed to write outputs", "path", path, "error", err.Error())
		return
	}
	log.Info("wrote outputs", append(attrs(out), slog.String("path", path), slog.String("updated_at", out.UpdatedAt))...)
}

func attrs(out result) []any {
	return []any{
		"status", out.Status,
		"checks", out.Checks,
		"domain", out.Domain,
		"delegation_status", out.DelegationStatus,
		"expected_nameservers", out.ExpectedNameservers,
		"observed_nameservers", out.ObservedNameservers,
		"delegated", out.Delegated,
		"soa_status", out.SOAStatus,
		"caa_status", out.CAAStatus,
		"ds_status", out.DSStatus,
		"diagnosis", out.Diagnosis,
	}
}

func appendFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func parseChecks(raw string) []string {
	want := map[string]bool{}
	for _, f := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
		f = strings.ToLower(strings.TrimSpace(f))
		for _, k := range knownChecks {
			if f == k {
				want[k] = true
			}
		}
	}
	if len(want) == 0 {
		return []string{checkDelegation}
	}
	var out []string
	for _, k := range knownChecks {
		if want[k] {
			out = append(out, k)
		}
	}
	return out
}

func normalize(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(f), "."))
		if f == "" {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func nsMatch(observed, expected []string, strict bool) bool {
	if strict {
		return len(observed) == len(expected) && containsAll(observed, expected)
	}
	return containsAll(observed, expected)
}

func containsAll(haystack, needles []string) bool {
	set := make(map[string]struct{}, len(haystack))
	for _, h := range haystack {
		set[h] = struct{}{}
	}
	for _, n := range needles {
		if _, ok := set[n]; !ok {
			return false
		}
	}
	return len(needles) > 0
}

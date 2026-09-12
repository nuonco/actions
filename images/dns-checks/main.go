package main

import (
	"flag"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("dns-checks", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	domain := fs.String("domain", "", "public zone name to check (env DOMAIN)")
	nameservers := fs.String("nameservers", "", "comma-separated expected NS hosts (env NAMESERVERS)")
	output := fs.String("output", "", "file to append JSON outputs to (env NUON_ACTIONS_OUTPUT_FILEPATH)")
	attempts := fs.String("attempts", "", "times to retrace a missing or mismatched delegation (env ATTEMPTS)")
	sleepSeconds := fs.String("sleep-seconds", "", "seconds between attempts (env SLEEP_SECONDS)")
	checks := fs.String("checks", "", "comma-separated checks to run: delegation,soa,caa,ds (env CHECKS)")
	strictNS := fs.String("strict-nameservers", "", "require the parent NS set to match exactly (env STRICT_NAMESERVERS)")
	caaIssuers := fs.String("caa-issuers", "", "CAA issue tags that must be present (env CAA_ISSUERS)")
	if err := fs.Parse(args); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("invalid flags", "error", err.Error())
		return 0
	}

	cfg := config{
		domain:      resolve(*domain, "DOMAIN", ""),
		nameservers: resolve(*nameservers, "NAMESERVERS", ""),
		output:      resolve(*output, "NUON_ACTIONS_OUTPUT_FILEPATH", "/dev/stdout"),
		attempts:    resolveInt(*attempts, "ATTEMPTS", 3),
		sleep:       time.Duration(resolveInt(*sleepSeconds, "SLEEP_SECONDS", 20)) * time.Second,
		checks:      parseChecks(resolve(*checks, "CHECKS", "")),
		strictNS:    resolveBool(*strictNS, "STRICT_NAMESERVERS", false),
		caaIssuers:  normalize(resolve(*caaIssuers, "CAA_ISSUERS", "letsencrypt.org")),
	}
	check(os.Stdout, cfg, liveClient())
	return 0
}

func resolve(flagVal, envName, fallback string) string {
	v := strings.TrimSpace(flagVal)
	if v != "" && !isEnvRef(v, envName) {
		return v
	}
	if env := os.Getenv(envName); env != "" {
		return env
	}
	return fallback
}

func resolveBool(flagVal, envName string, fallback bool) bool {
	v := strings.ToLower(resolve(flagVal, envName, ""))
	switch v {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}

func resolveInt(flagVal, envName string, fallback int) int {
	v := resolve(flagVal, envName, "")
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func isEnvRef(v, envName string) bool {
	return v == "$"+envName || v == "${"+envName+"}"
}

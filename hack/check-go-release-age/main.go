// Supply-chain protection: refuse Go modules younger than minReleaseAge.
//
// Go has no native equivalent of pnpm's `minimumReleaseAge`, so this tool
// reads go.sum, queries proxy.golang.org for each module's publish time,
// and fails CI if any pinned version is younger than the cooldown window.
// Exceptions live in .supply-chain/go-allowlist.yaml.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultMinReleaseAge = 7 * 24 * time.Hour
	defaultGoSum         = "go.sum"
	defaultAllowlist     = ".supply-chain/go-allowlist.yaml"
	defaultProxy         = "https://proxy.golang.org"
	defaultConcurrency   = 10
	requestTimeout       = 15 * time.Second
)

// errViolations is returned by run when the audit found release-age
// violations. main() maps it to exit code 1 (policy failure) and any other
// error to exit code 2 (setup/lookup failure) so CI can distinguish them.
var errViolations = errors.New("release-age violations detected")

type modulePin struct {
	Module  string
	Version string
}

type allowlistEntry struct {
	Module     string `yaml:"module"`
	Version    string `yaml:"version"`
	Reason     string `yaml:"reason"`
	ApprovedBy string `yaml:"approved_by"`
	Added      string `yaml:"added"`
	Expires    string `yaml:"expires"`
}

type allowlist struct {
	Exclude []allowlistEntry `yaml:"exclude"`
}

type violation struct {
	modulePin
	PublishedAt time.Time
	AgeDays     float64
}

type proxyInfo struct {
	Version string    `json:"Version"`
	Time    time.Time `json:"Time"`
}

func main() {
	var (
		goSumPath   = flag.String("go-sum", defaultGoSum, "path to go.sum")
		allowPath   = flag.String("allowlist", defaultAllowlist, "path to allowlist YAML")
		proxy       = flag.String("proxy", defaultProxy, "Go module proxy base URL")
		minAgeFlag  = flag.Duration("min-age", defaultMinReleaseAge, "minimum module release age")
		concurrency = flag.Int("concurrency", defaultConcurrency, "max parallel proxy requests")
	)
	flag.Parse()

	err := run(*goSumPath, *allowPath, *proxy, *minAgeFlag, *concurrency)
	if err == nil {
		return
	}
	if errors.Is(err, errViolations) {
		// run() already printed the violation table; exit non-zero quietly.
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}

func run(goSumPath, allowPath, proxy string, minAge time.Duration, concurrency int) error {
	pins, err := parseGoSum(goSumPath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", goSumPath, err)
	}

	allow, allowWarnings, err := loadAllowlist(allowPath)
	if err != nil {
		return fmt.Errorf("load allowlist: %w", err)
	}
	for _, w := range allowWarnings {
		fmt.Fprintf(os.Stderr, "warn: %s\n", w)
	}

	now := time.Now()
	filtered := make([]modulePin, 0, len(pins))
	excluded := 0
	for _, p := range pins {
		if allow.matches(p, now) {
			excluded++
			continue
		}
		filtered = append(filtered, p)
	}

	fmt.Printf("Auditing %d unique modules in %s (%d excluded). Minimum age: %s.\n",
		len(filtered), goSumPath, excluded, minAge)

	cutoff := now.Add(-minAge)
	violations, errs := fetchViolations(filtered, proxy, cutoff, concurrency)

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d lookup error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  ! %s\n", e)
		}
		return errors.New("lookup errors prevented a complete audit")
	}

	if len(violations) == 0 {
		fmt.Println("OK: all modules meet the minimum release age.")
		return nil
	}

	sort.Slice(violations, func(i, j int) bool {
		return violations[i].AgeDays < violations[j].AgeDays
	})
	fmt.Fprintf(os.Stderr, "\n%d violation(s):\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  - %s@%s  published %s  (%.2f days old)\n",
			v.Module, v.Version, v.PublishedAt.Format(time.RFC3339), v.AgeDays)
	}
	fmt.Fprintln(os.Stderr,
		"\nTo bypass for a legitimate hotfix, add the module to .supply-chain/go-allowlist.yaml "+
			"with a justification (reason, approver, expiry).")
	return errViolations
}

func parseGoSum(path string) ([]modulePin, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	seen := make(map[string]struct{})
	var pins []modulePin
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 2 {
			continue
		}
		// Drop the `/go.mod` suffix on the second-line variant so we
		// dedupe to one entry per <module, version> pair.
		version := strings.TrimSuffix(fields[1], "/go.mod")
		key := fields[0] + "@" + version
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		pins = append(pins, modulePin{Module: fields[0], Version: version})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return pins, nil
}

func loadAllowlist(path string) (*allowlist, []string, error) {
	a := &allowlist{}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return a, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if err := yaml.Unmarshal(raw, a); err != nil {
		return nil, nil, err
	}
	var warnings []string
	now := time.Now()
	for _, e := range a.Exclude {
		if e.Expires == "" {
			warnings = append(warnings,
				fmt.Sprintf("allowlist entry %s@%s has no `expires` date: every exception must expire",
					e.Module, e.Version))
			continue
		}
		exp, err := time.Parse("2006-01-02", e.Expires)
		if err != nil {
			warnings = append(warnings,
				fmt.Sprintf("allowlist entry %s@%s: unparseable expires %q (want YYYY-MM-DD)",
					e.Module, e.Version, e.Expires))
			continue
		}
		if now.After(exp) {
			warnings = append(warnings,
				fmt.Sprintf("allowlist entry %s@%s expired on %s: remove or renew",
					e.Module, e.Version, e.Expires))
		}
	}
	return a, warnings, nil
}

// matches reports whether p is allowed to bypass the release-age cooldown.
// Fails closed: an entry whose `expires` is missing, unparseable, or past
// `now` is ignored, so a stale bypass stops bypassing. The existing
// `loadAllowlist` warnings flag the same entries for human follow-up.
func (a *allowlist) matches(p modulePin, now time.Time) bool {
	for _, e := range a.Exclude {
		if e.Module != p.Module {
			continue
		}
		if e.Version != "" && e.Version != p.Version {
			continue
		}
		exp, err := time.Parse("2006-01-02", e.Expires)
		if err != nil || now.After(exp) {
			continue
		}
		return true
	}
	return false
}

func fetchViolations(pins []modulePin, proxy string, cutoff time.Time, concurrency int) ([]violation, []string) {
	if concurrency < 1 {
		concurrency = 1
	}
	client := &http.Client{Timeout: requestTimeout}
	jobs := make(chan modulePin)
	var mu sync.Mutex
	var violations []violation
	var errs []string
	var wg sync.WaitGroup

	for range concurrency {
		wg.Go(func() {
			for p := range jobs {
				info, err := fetchInfo(client, proxy, p)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("%s@%s: %v", p.Module, p.Version, err))
					mu.Unlock()
					continue
				}
				if info.Time.After(cutoff) {
					mu.Lock()
					violations = append(violations, violation{
						modulePin:   p,
						PublishedAt: info.Time,
						AgeDays:     time.Since(info.Time).Hours() / 24,
					})
					mu.Unlock()
				}
			}
		})
	}
	for _, p := range pins {
		jobs <- p
	}
	close(jobs)
	wg.Wait()
	return violations, errs
}

func fetchInfo(client *http.Client, proxy string, p modulePin) (*proxyInfo, error) {
	url := fmt.Sprintf("%s/%s/@v/%s.info", strings.TrimRight(proxy, "/"), escapePath(p.Module), escapePath(p.Version))
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var info proxyInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode proxy response: %w", err)
	}
	if info.Time.IsZero() {
		return nil, errors.New("proxy returned empty Time")
	}
	return &info, nil
}

// escapePath lowercases uppercase letters per the Go module proxy protocol
// (`A`-`Z` -> `!a`-`!z`). See https://go.dev/ref/mod#module-proxy.
func escapePath(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

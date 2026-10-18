// Command admin manages job sources from the command line:
//
//	admin sources list
//	admin sources add -key KEY -name NAME -adapter greenhouse -config '{"board_token":"acme"}' [-interval 6h] [-base-url URL]
//	admin sources review -key KEY -note "what was checked, link to terms"
//	admin sources enable|disable -key KEY
//	admin sources sync -key KEY
//	admin sources runs -key KEY [-limit 10]
//	admin adapters list
//	admin adapters review -adapter NAME -note "what was checked"
//	admin adapters enable|disable -adapter NAME
//	admin companies list [-status STATUS] [-q TEXT]
//	admin companies add -url CAREERS_URL [-name NAME]
//	admin seeds check [-file FILE] [-write GOOD_FILE]
//	admin llm check | eval [-set DIR] [-model NAME] [-limit N] | enrich [-limit N]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/discovery"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/adapters"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `usage: admin sources <list|add|review|enable|disable|sync|runs> [flags]
       admin adapters <list|review|enable|disable> [flags]
       admin companies <list|add> [flags]
       admin seeds check [-file FILE] [-write GOOD_FILE]
       admin llm <check|eval|enrich> [flags]
       admin version

Sources start disabled. A source can be enabled when it has its own review
("admin sources review") or its adapter type has an enabled review ("admin
adapters review", then "admin adapters enable"). See docs/sources/README.md.`

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(out, version)
		return nil
	}
	if len(args) >= 1 && args[0] == "llm" {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return llmCommand(ctx, cfg, args[1:], out)
	}
	if len(args) < 2 || (args[0] != "sources" && args[0] != "adapters" && args[0] != "companies" && args[0] != "seeds") {
		return errors.New(usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if args[0] == "seeds" {
		return seedsCommand(ctx, cfg, args[1:], out)
	}
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	c := &cli{
		sources:   database.NewSourceRepository(pool),
		jobs:      database.NewJobRepository(pool),
		companies: database.NewCompanyRepository(pool),
		adapters:  adapters.Registry(adapters.NewCrawler(version, cfg.Crawl), adapters.Options{JSONLDMaxPages: cfg.Crawl.JSONLDMaxPages}),
		out:       out,
	}
	switch args[0] {
	case "adapters":
		return c.adapterCommand(ctx, args[1], args[2:])
	case "companies":
		return c.companyCommand(ctx, args[1], args[2:])
	}
	return c.dispatch(ctx, args[1], args[2:])
}

type cli struct {
	sources   *database.SourceRepository
	jobs      *database.JobRepository
	companies *database.CompanyRepository
	adapters  ingest.Registry
	out       io.Writer
}

func (c *cli) adapterCommand(ctx context.Context, cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	name := fs.String("adapter", "", "adapter type, such as workday")
	note := fs.String("note", "", "what was reviewed and where the terms are")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cmd == "list" {
		reviews, err := c.companies.AdapterReviews(ctx)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(c.out, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "ADAPTER\tENABLED\tREVIEWED\tNOTE")
		for _, r := range reviews {
			reviewed, text := "no", ""
			if r.ReviewedAt != nil {
				reviewed = r.ReviewedAt.Format("2006-01-02")
			}
			if r.Note != nil {
				text = *r.Note
			}
			fmt.Fprintf(w, "%s\t%v\t%s\t%s\n", r.Adapter, r.Enabled, reviewed, text)
		}
		return w.Flush()
	}
	if err := required(*name, "adapter"); err != nil {
		return err
	}
	var current *domain.AdapterReview
	reviews, err := c.companies.AdapterReviews(ctx)
	if err != nil {
		return err
	}
	for _, r := range reviews {
		if r.Adapter == *name {
			current = &r
		}
	}
	switch cmd {
	case "review":
		if err := required(strings.TrimSpace(*note), "note"); err != nil {
			return err
		}
		enabled := current != nil && current.Enabled
		if _, err := c.companies.ReviewAdapter(ctx, *name, *note, enabled); err != nil {
			return err
		}
		fmt.Fprintf(c.out, "Recorded review for adapter %s. Enable it with: admin adapters enable -adapter %s\n", *name, *name)
		return nil
	case "enable", "disable":
		enable := cmd == "enable"
		existing := ""
		if current != nil && current.Note != nil && current.ReviewedAt != nil {
			existing = *current.Note
		}
		if enable && existing == "" {
			return fmt.Errorf("adapter %s has no review; run: admin adapters review -adapter %s -note \"...\"", *name, *name)
		}
		if _, err := c.companies.ReviewAdapter(ctx, *name, existing, enable); err != nil {
			return err
		}
		fmt.Fprintf(c.out, "Adapter %s is now %s.\n", *name, map[bool]string{true: "enabled", false: "disabled"}[enable])
		return nil
	}
	return errors.New(usage)
}

func (c *cli) companyCommand(ctx context.Context, cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	switch cmd {
	case "list":
		status := fs.String("status", "", "pending, active, empty, unreachable, blocked_by_robots or disabled")
		q := fs.String("q", "", "text in the name or careers URL")
		limit := fs.Int("limit", 100, "number of companies")
		if err := fs.Parse(args); err != nil {
			return err
		}
		list, _, err := c.companies.List(ctx, database.CompanyFilter{Query: *q, Status: domain.CompanyStatus(*status), Limit: *limit})
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(c.out, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tATS\tSTATUS\tJOBS\tCAREERS URL")
		for _, co := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", co.Name, co.ATS, co.Status, co.OpenJobs, co.CareersURL)
		}
		return w.Flush()
	case "add":
		rawURL := fs.String("url", "", "career page URL")
		name := fs.String("name", "", "company name (defaults to the host)")
		if err := fs.Parse(args); err != nil {
			return err
		}
		u, err := url.Parse(*rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("-url must be an absolute http(s) URL")
		}
		if *name == "" {
			*name = strings.TrimPrefix(u.Hostname(), "www.")
		}
		co, _, err := c.companies.UpsertDiscovered(ctx, database.CompanyParams{
			Name: *name, CareersURL: u.String(), ATS: "unknown", Origin: "manual",
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(c.out, "%s is %s; the worker detects its career page on its next tick.\n", co.Name, co.Status)
		return nil
	}
	return errors.New(usage)
}

func (c *cli) dispatch(ctx context.Context, cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	key := fs.String("key", "", "source key, such as greenhouse-acme")
	switch cmd {
	case "list":
		if err := fs.Parse(args); err != nil {
			return err
		}
		return c.list(ctx)
	case "add":
		name := fs.String("name", "", "display name")
		adapter := fs.String("adapter", "", "adapter: "+strings.Join(c.adapters.Names(), ", "))
		rawConfig := fs.String("config", "{}", "adapter configuration as a JSON object")
		interval := fs.Duration("interval", 6*time.Hour, "sync interval (at least 5m)")
		baseURL := fs.String("base-url", "", "public URL of the source, for attribution")
		if err := fs.Parse(args); err != nil {
			return err
		}
		return c.add(ctx, *key, *name, *adapter, *rawConfig, *interval, *baseURL)
	case "review":
		note := fs.String("note", "", "what was reviewed and where the terms are")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if err := required(*key, "key"); err != nil {
			return err
		}
		if err := required(strings.TrimSpace(*note), "note"); err != nil {
			return err
		}
		s, err := c.sources.Review(ctx, *key, strings.TrimSpace(*note))
		if err != nil {
			return err
		}
		fmt.Fprintf(c.out, "Recorded review for %s at %s.\n", s.Key, s.ReviewedAt.Format(time.RFC3339))
		return nil
	case "enable", "disable":
		if err := fs.Parse(args); err != nil {
			return err
		}
		if err := required(*key, "key"); err != nil {
			return err
		}
		s, err := c.sources.SetEnabled(ctx, *key, cmd == "enable")
		if err != nil {
			return err
		}
		fmt.Fprintf(c.out, "%s is now %s.\n", s.Key, map[bool]string{true: "enabled", false: "disabled"}[s.Enabled])
		return nil
	case "sync":
		if err := fs.Parse(args); err != nil {
			return err
		}
		return c.sync(ctx, *key)
	case "runs":
		limit := fs.Int("limit", 10, "number of runs")
		if err := fs.Parse(args); err != nil {
			return err
		}
		return c.runs(ctx, *key, *limit)
	default:
		return errors.New(usage)
	}
}

func required(v, name string) error {
	if v == "" {
		return fmt.Errorf("-%s is required", name)
	}
	return nil
}

func (c *cli) list(ctx context.Context) error {
	sources, err := c.sources.List(ctx)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(c.out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tADAPTER\tENABLED\tREVIEWED\tINTERVAL\tLAST RUN")
	for _, s := range sources {
		reviewed := "no"
		if s.ReviewedAt != nil {
			reviewed = s.ReviewedAt.Format("2006-01-02")
		}
		last := "never"
		if runs, err := c.sources.ListRuns(ctx, s.ID, 1); err != nil {
			return err
		} else if len(runs) > 0 {
			last = fmt.Sprintf("%s %s", runs[0].Status, runs[0].StartedAt.Format(time.RFC3339))
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\t%s\t%s\n", s.Key, s.Adapter, s.Enabled, reviewed, s.SyncInterval, last)
	}
	return w.Flush()
}

func (c *cli) add(ctx context.Context, key, name, adapterName, rawConfig string, interval time.Duration, baseURL string) error {
	for _, f := range []struct{ v, name string }{{key, "key"}, {name, "name"}, {adapterName, "adapter"}, {baseURL, "base-url"}} {
		if err := required(f.v, f.name); err != nil {
			return err
		}
	}
	adapter, err := c.adapters.Get(adapterName)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(rawConfig), &cfg); err != nil || cfg == nil {
		return errors.New("-config must be a JSON object")
	}
	if err := adapter.Validate(cfg); err != nil {
		return fmt.Errorf("invalid -config: %w", err)
	}
	if interval < 5*time.Minute {
		return errors.New("-interval must be at least 5m")
	}
	s, err := c.sources.Create(ctx, database.SourceParams{
		Key: key, Name: name, BaseURL: baseURL, Adapter: adapterName, Config: cfg, SyncInterval: interval,
	})
	if errors.Is(err, domain.ErrConflict) {
		return fmt.Errorf("a source with key %q already exists", key)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Added %s (disabled). Review it before enabling:\n  admin sources review -key %s -note \"...\"\n", s.Key, s.Key)
	return nil
}

// sync runs one source now, even if it is disabled, so an operator can test
// a reviewed configuration before enabling scheduled syncs.
func (c *cli) sync(ctx context.Context, key string) error {
	if err := required(key, "key"); err != nil {
		return err
	}
	s, err := c.sources.Get(ctx, key)
	if err != nil {
		return err
	}
	if s.ReviewedAt == nil {
		return database.ErrSourceNotReviewed
	}
	runner := &ingest.Runner{
		Sources:   c.sources,
		Listings:  c.jobs,
		Companies: c.companies,
		Adapters:  c.adapters,
		Logger:    slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
	run, err := runner.Sync(ctx, s)
	printRun(c.out, run)
	return err
}

func (c *cli) runs(ctx context.Context, key string, limit int) error {
	if err := required(key, "key"); err != nil {
		return err
	}
	if limit < 1 || limit > 1000 {
		return errors.New("-limit must be between 1 and 1000")
	}
	s, err := c.sources.Get(ctx, key)
	if err != nil {
		return err
	}
	runs, err := c.sources.ListRuns(ctx, s.ID, limit)
	if err != nil {
		return err
	}
	for _, r := range runs {
		printRun(c.out, r)
	}
	return nil
}

func printRun(w io.Writer, r domain.IngestionRun) {
	if r.ID == "" {
		return
	}
	fmt.Fprintf(w, "%s  %-9s  attempts=%d fetched=%d upserted=%d failed=%d",
		r.StartedAt.Format(time.RFC3339), r.Status, r.Attempts, r.FetchedCount, r.UpsertedCount, r.FailedCount)
	if r.Error != nil {
		fmt.Fprintf(w, "  error=%q", *r.Error)
	}
	fmt.Fprintln(w)
}

// seedsCommand checks that seed career pages lead to a supported board with
// jobs, so dead entries can be removed before a release.
func seedsCommand(ctx context.Context, cfg config.Config, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "check" {
		return errors.New(usage)
	}
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	file := fs.String("file", "", "seed file to check (default: the embedded list)")
	write := fs.String("write", "", "write the entries that passed to this file")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	var seeds []discovery.Seed
	var err error
	if *file == "" {
		seeds, err = discovery.Seeds()
	} else {
		var data []byte
		if data, err = os.ReadFile(*file); err == nil {
			seeds, err = discovery.ParseSeeds(string(data))
		}
	}
	if err != nil {
		return err
	}
	crawler := adapters.NewCrawler(version, cfg.Crawl)
	registry := adapters.Registry(crawler, adapters.Options{JSONLDMaxPages: 5})
	detector := &discovery.Detector{Client: crawler}

	type result struct {
		line string
		ok   bool
		msg  string
	}
	results := make([]result, len(seeds))
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Crawl.Concurrency)
	for i, s := range seeds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			line := s.URL
			if s.Name != "" {
				line += " | " + s.Name
			}
			results[i] = result{line: line}
			det, err := detector.Detect(ctx, s.URL)
			if err != nil {
				results[i].msg = err.Error()
				return
			}
			if det.Board.ATS == "unknown" {
				results[i].msg = "no supported job board found"
				return
			}
			adapter, err := registry.Get(det.Board.ATS)
			if err != nil {
				results[i].msg = err.Error()
				return
			}
			fetchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			jobs, err := adapter.Fetch(fetchCtx, det.Board.Config)
			cancel()
			if err != nil {
				results[i].msg = det.Board.ATS + ": " + err.Error()
				return
			}
			// Some systems answer unknown boards with an empty list, so a
			// seed must have at least one open job to prove it is real.
			if len(jobs) == 0 {
				results[i].msg = det.Board.ATS + ": no open jobs"
				return
			}
			results[i].ok = true
			results[i].msg = fmt.Sprintf("%-10s %4d jobs", det.Board.ATS, len(jobs))
		}()
	}
	wg.Wait()

	var good strings.Builder
	passed := 0
	for _, r := range results {
		if r.ok {
			passed++
			good.WriteString(r.line + "\n")
			fmt.Fprintf(out, "OK   %s  %s\n", r.msg, r.line)
		} else {
			fmt.Fprintf(out, "DEAD %s  (%s)\n", r.line, r.msg)
		}
	}
	fmt.Fprintf(out, "%d of %d seeds lead to a supported job board.\n", passed, len(seeds))
	if *write != "" {
		return os.WriteFile(*write, []byte(good.String()), 0o644)
	}
	return nil
}

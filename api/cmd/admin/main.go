// Command admin manages job sources from the command line:
//
//	admin sources list
//	admin sources add -key KEY -name NAME -adapter greenhouse -config '{"board_token":"acme"}' [-interval 6h] [-base-url URL]
//	admin sources review -key KEY -note "what was checked, link to terms"
//	admin sources enable|disable -key KEY
//	admin sources sync -key KEY
//	admin sources runs -key KEY [-limit 10]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/adapters"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `usage: admin sources <list|add|review|enable|disable|sync|runs> [flags]
       admin version

Sources start disabled. Before enabling one, review its access method,
permissions and terms, and record the result with "admin sources review".
See docs/sources/README.md.`

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
	if len(args) < 2 || args[0] != "sources" {
		return errors.New(usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	c := &cli{
		sources:  database.NewSourceRepository(pool),
		jobs:     database.NewJobRepository(pool),
		adapters: adapters.Registry(),
		out:      out,
	}
	return c.dispatch(ctx, args[1], args[2:])
}

type cli struct {
	sources  *database.SourceRepository
	jobs     *database.JobRepository
	adapters ingest.Registry
	out      io.Writer
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
		Sources:  c.sources,
		Listings: c.jobs,
		Adapters: c.adapters,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
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

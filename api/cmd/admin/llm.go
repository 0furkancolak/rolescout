package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/enrich"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/llm"
	"example.com/rolescout/api/internal/llm/eval"
)

const llmUsage = `usage: admin llm check
       admin llm eval [-set DIR_OR_FILE] [-model NAME] [-limit N]
       admin llm enrich [-limit N]

check  tests the model server from LLM_* settings (list models, one tiny request).
eval   runs the labelled descriptions through the model and prints accuracy and speed.
enrich reads open jobs with the model now, newest and best-matching first (needs the database).`

// llmCommand runs the model tools. They use the LLM_* settings even when
// LLM_ENABLED is false, so a model can be tried before it is switched on.
func llmCommand(ctx context.Context, cfg config.Config, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(llmUsage)
	}
	client := llm.NewClient(llm.Options{BaseURL: cfg.LLM.BaseURL, Model: cfg.LLM.Model, APIKey: cfg.LLM.APIKey, Timeout: cfg.LLM.Timeout})
	switch args[0] {
	case "check":
		return llmCheck(ctx, cfg, client, out)
	case "eval":
		return llmEval(ctx, cfg, args[1:], out)
	case "enrich":
		return llmEnrich(ctx, cfg, args[1:], out)
	}
	return errors.New(llmUsage)
}

func llmCheck(ctx context.Context, cfg config.Config, client *llm.Client, out io.Writer) error {
	fmt.Fprintf(out, "server: %s\nmodel: %s\n", hostOnlyURL(cfg.LLM.BaseURL), cfg.LLM.Model)
	h, err := client.Health(ctx)
	if err != nil {
		fmt.Fprintln(out, "reachable: no")
		return fmt.Errorf("model server is not reachable: %w", err)
	}
	fmt.Fprintf(out, "reachable: yes (%s)\n", h.Latency.Round(time.Millisecond))
	if !h.ModelPresent {
		fmt.Fprintln(out, "model present: no")
		return fmt.Errorf("model %q is not on the server; run: ollama pull %s", cfg.LLM.Model, cfg.LLM.Model)
	}
	fmt.Fprintln(out, "model present: yes")
	resp, err := client.Probe(ctx)
	if err != nil {
		return fmt.Errorf("the model did not answer: %w", err)
	}
	fmt.Fprintf(out, "answered in %s\n", resp.Latency.Round(time.Millisecond))
	return nil
}

func llmEval(ctx context.Context, cfg config.Config, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	set := fs.String("set", "internal/llm/testdata/eval", "directory or JSON file of labelled cases")
	model := fs.String("model", cfg.LLM.Model, "model to evaluate")
	limit := fs.Int("limit", 0, "evaluate only the first N cases (0 = all)")
	quiet := fs.Bool("q", false, "no progress lines")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cases, err := eval.LoadCases(*set)
	if err != nil {
		return err
	}
	if *limit > 0 && *limit < len(cases) {
		cases = cases[:*limit]
	}
	client := llm.NewClient(llm.Options{BaseURL: cfg.LLM.BaseURL, Model: *model, APIKey: cfg.LLM.APIKey, Timeout: cfg.LLM.Timeout})
	rep := eval.Run(ctx, client, cases, eval.Options{
		Model: *model, MaxChars: cfg.LLM.MaxInputChars, MaxTokens: cfg.LLM.MaxOutputTokens, ResolveCountry: ingest.InferCountry,
		Progress: func(done, total int, c eval.Case, err error, took time.Duration) {
			if *quiet {
				return
			}
			status := "ok"
			if err != nil {
				status = "FAILED"
			}
			fmt.Fprintf(os.Stderr, "[%d/%d] %s %s %.1fs\n", done, total, c.ID, status, took.Seconds())
		},
	})
	fmt.Fprint(out, rep.Format())
	return nil
}

func llmEnrich(ctx context.Context, cfg config.Config, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("enrich", flag.ContinueOnError)
	limit := fs.Int("limit", 25, "how many jobs to read")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	e := &enrich.Enricher{
		Repo:  database.NewEnrichmentRepository(pool),
		LLM:   llm.NewClient(llm.Options{BaseURL: cfg.LLM.BaseURL, Model: cfg.LLM.Model, APIKey: cfg.LLM.APIKey, Timeout: cfg.LLM.Timeout}),
		Model: cfg.LLM.Model, Concurrency: cfg.LLM.Concurrency, Batch: *limit, MaxChars: cfg.LLM.MaxInputChars, MaxTokens: cfg.LLM.MaxOutputTokens,
		Breaker: enrich.NewBreaker(3, time.Minute, nil), Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		ResolveCountry: ingest.InferCountry,
	}
	start := time.Now()
	res, err := e.Tick(ctx)
	fmt.Fprintf(out, "enriched %d, failed %d in %s\n", res.Enriched, res.Failed, time.Since(start).Round(time.Second))
	return err
}

func hostOnlyURL(raw string) string {
	var host string
	fmt.Sscanf(raw, "http://%[^/]", &host)
	if host == "" {
		fmt.Sscanf(raw, "https://%[^/]", &host)
	}
	return host
}

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/lib/pq"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/policy"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

func main() {
	output := flag.String("output", "", "output file path (defaults to stdout)")
	flag.Parse()

	var cfg config.Config
	if err := envconfig.Process("TRUSTGRAPH", &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	db, err := store.NewPostgres(context.Background(), cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Load all reviewed assessments as simulation inputs.
	rows, err := db.QueryContext(context.Background(), `
		SELECT a.risk_score, r.outcome
		FROM assessment a
		JOIN assessment_review r ON a.assessment_id = r.assessment_id
		WHERE r.outcome IN ('confirmed_abuse','legitimate')
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var inputs []policy.SimulationInput
	for rows.Next() {
		var score int
		var outcome string
		var _ pq.StringArray
		var _ sql.NullTime
		if err := rows.Scan(&score, &outcome); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			os.Exit(1)
		}
		inputs = append(inputs, policy.SimulationInput{
			RiskScore: score,
			Outcome:   models.ReviewOutcome(outcome),
		})
	}

	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "no reviewed assessments found — collect at least 50 before simulating")
		os.Exit(1)
	}

	thresholds := []int{50, 60, 70, 80}
	results := policy.SimulatePolicy(inputs, thresholds)

	out := os.Stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open output: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}

	// Print gate summary to stderr regardless of output flag.
	fmt.Fprintf(os.Stderr, "\n%d assessments simulated across thresholds %v\n", len(inputs), thresholds)
	fmt.Fprintf(os.Stderr, "Gate to enforcement: FP rate < 0.10 AND FN rate < 0.20\n\n")
	for _, r := range results {
		gate := "✗"
		if r.FalsePositiveRate < 0.10 && r.FalseNegativeRate < 0.20 {
			gate = "✓"
		}
		fmt.Fprintf(os.Stderr, "  threshold=%d  FP=%.2f  FN=%.2f  precision=%.2f  recall=%.2f  %s\n",
			r.Threshold, r.FalsePositiveRate, r.FalseNegativeRate, r.Precision, r.Recall, gate)
	}
}

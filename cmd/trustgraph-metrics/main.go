package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

func main() {
	format := flag.String("format", "json", "output format: json or csv")
	output := flag.String("output", "", "output file path (defaults to stdout)")
	flag.Parse()

	var cfg config.Config
	if err := envconfig.Process("TRUSTGRAPH", &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	db, err := store.NewPostgres(context.Background(), cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := store.NewCalibrationRepository(db)
	metrics, err := repo.GetCalibrationMetrics(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "metrics error: %v\n", err)
		os.Exit(1)
	}

	out := os.Stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	switch *format {
	case "csv":
		w := csv.NewWriter(out)
		w.Write([]string{"metric", "value"})
		w.Write([]string{"total_reviews", fmt.Sprintf("%d", metrics.TotalReviews)})
		w.Write([]string{"false_positive_rate", fmt.Sprintf("%.4f", metrics.FalsePositiveRate)})
		w.Write([]string{"false_negative_rate", fmt.Sprintf("%.4f", metrics.FalseNegativeRate)})
		w.Write([]string{"appeal_overturn_rate", fmt.Sprintf("%.4f", metrics.AppealOverturnRate)})
		for band, stats := range metrics.ByRiskBand {
			w.Write([]string{fmt.Sprintf("band_%s_reviews", band), fmt.Sprintf("%d", stats.Reviews)})
			w.Write([]string{fmt.Sprintf("band_%s_abuse_rate", band), fmt.Sprintf("%.4f", stats.AbuseRate)})
		}
		for code, stats := range metrics.ByReasonCode {
			w.Write([]string{fmt.Sprintf("code_%s_precision", code), fmt.Sprintf("%.4f", stats.Precision)})
			w.Write([]string{fmt.Sprintf("code_%s_recall", code), fmt.Sprintf("%.4f", stats.Recall)})
		}
		w.Flush()
	default:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(metrics); err != nil {
			fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
			os.Exit(1)
		}
	}
}

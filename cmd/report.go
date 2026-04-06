package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/0xmhha/commitflow/internal/analysis"
	"github.com/0xmhha/commitflow/internal/config"
	"github.com/0xmhha/commitflow/internal/storage"
	"github.com/spf13/cobra"
)

const (
	summaryMaxLen  = 60
	defaultLimit   = 50
	reportHashLen  = 8
)

var (
	reportCategory  string
	reportSince     string
	reportImpactMin int
	reportLimit     int
)

var reportCmd = &cobra.Command{
	Use:   "report [repo-path]",
	Short: "Generate analysis report",
	Long:  "Display a tabular report of analyzed commits with optional filtering.",
	Args:  cobra.ExactArgs(1),
	RunE:  runReport,
}

func init() {
	reportCmd.Flags().StringVar(&reportCategory, "category", "", "filter by category (e.g. bug_fix, feature)")
	reportCmd.Flags().StringVar(&reportSince, "since", "", "filter commits after date (YYYY-MM-DD)")
	reportCmd.Flags().IntVar(&reportImpactMin, "impact-min", 0, "minimum impact score (1-10)")
	reportCmd.Flags().IntVar(&reportLimit, "limit", defaultLimit, "maximum number of results to show")

	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
	repoPath := args[0]
	if repoPath == "" {
		return fmt.Errorf("repo-path must not be empty")
	}

	if err := config.EnsureDBDir(appConfig.DBPath); err != nil {
		return fmt.Errorf("ensure db directory: %w", err)
	}

	db, err := storage.OpenDB(appConfig.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	store := storage.NewCommitStore(db)
	ctx := cmd.Context()

	filter := buildReportFilter(repoPath)
	analyses, err := store.ListAnalyses(ctx, filter)
	if err != nil {
		return fmt.Errorf("list analyses: %w", err)
	}

	categoryCounts, err := store.CountByCategory(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("count by category: %w", err)
	}

	printReportTable(analyses)
	printReportStats(len(analyses), categoryCounts)
	return nil
}

func buildReportFilter(repoPath string) storage.AnalysisFilter {
	return storage.AnalysisFilter{
		RepoPath:  repoPath,
		Category:  reportCategory,
		Since:     reportSince,
		MinImpact: reportImpactMin,
		Limit:     reportLimit,
	}
}

func printReportTable(analyses []storage.CommitAnalysis) {
	if len(analyses) == 0 {
		fmt.Println("No analyses found matching the given filters.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "HASH\tCATEGORY\tIMPACT\tDATE\tSUMMARY")
	fmt.Fprintln(w, "----\t--------\t------\t----\t-------")

	for _, a := range analyses {
		hash := shortHashN(a.CommitHash, reportHashLen)
		label := analysis.CategoryLabel(a.Category)
		impact := fmt.Sprintf("%d/10", a.ImpactScore)
		date := shortDate(a.CommitDate)
		summary := truncateSummary(a.Summary, summaryMaxLen)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", hash, label, impact, date, summary)
	}
}

func printReportStats(total int, categoryCounts map[string]int) {
	fmt.Printf("\nTotal: %d commits\n", total)

	if len(categoryCounts) == 0 {
		return
	}

	fmt.Println("\nCategory breakdown (all time):")

	type catCount struct {
		category string
		count    int
	}
	sorted := make([]catCount, 0, len(categoryCounts))
	for cat, count := range categoryCounts {
		sorted = append(sorted, catCount{cat, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	for _, cc := range sorted {
		label := analysis.CategoryLabel(cc.category)
		fmt.Fprintf(w, "  %-10s %-20s %d\n", label, cc.category, cc.count)
	}
}

func shortHashN(hash string, n int) string {
	if len(hash) > n {
		return hash[:n]
	}
	return hash
}

func shortDate(dateStr string) string {
	if len(dateStr) >= 10 {
		return dateStr[:10]
	}
	return dateStr
}

func truncateSummary(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

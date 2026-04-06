package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/0xmhha/commitflow/internal/analysis"
	"github.com/0xmhha/commitflow/internal/config"
	"github.com/0xmhha/commitflow/internal/storage"
	"github.com/spf13/cobra"
)

var showRepo string

var showCmd = &cobra.Command{
	Use:   "show <commit-hash>",
	Short: "Show analysis for a specific commit",
	Long:  "Display the stored analysis result for a given commit hash. Use --repo to specify the repository.",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	showCmd.Flags().StringVar(&showRepo, "repo", "", "repository path (required)")
	_ = showCmd.MarkFlagRequired("repo")
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	commitHash := args[0]
	if commitHash == "" {
		return fmt.Errorf("commit-hash must not be empty")
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

	result, err := store.GetAnalysis(ctx, showRepo, commitHash)
	if err != nil {
		return fmt.Errorf("get analysis: %w", err)
	}

	if result == nil {
		return fmt.Errorf("no analysis found for commit %q in repo %q", commitHash, showRepo)
	}

	printCommitAnalysis(result)
	return nil
}

func printCommitAnalysis(a *storage.CommitAnalysis) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "Commit:\t%s\n", shortHash(a.CommitHash))
	fmt.Fprintf(w, "Author:\t%s <%s>\n", a.Author, a.AuthorEmail)
	fmt.Fprintf(w, "Date:\t%s\n", a.CommitDate)
	fmt.Fprintf(w, "Repository:\t%s\n", a.RepoPath)
	fmt.Fprintln(w, "")

	fmt.Fprintf(w, "Category:\t%s %s\n", analysis.CategoryLabel(a.Category), a.Category)
	fmt.Fprintf(w, "Impact:\t%d/10 (%s)\n", a.ImpactScore, analysis.ImpactLevel(a.ImpactScore))
	fmt.Fprintf(w, "Breaking Changes:\t%v\n", a.BreakingChanges)
	fmt.Fprintln(w, "")

	fmt.Fprintf(w, "Message:\t%s\n", firstLine(a.Message))
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "Summary:")
	fmt.Fprintln(w, indent(a.Summary, "  "))

	if a.DetailedAnalysis != "" {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Detailed Analysis:")
		fmt.Fprintln(w, indent(a.DetailedAnalysis, "  "))
	}

	if len(a.FilesChanged) > 0 {
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "Files Changed (%d):\n", len(a.FilesChanged))
		for _, f := range a.FilesChanged {
			fmt.Fprintf(w, "  %s\n", f)
		}
	}

	if a.BreakingChanges {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  WARNING: This commit contains breaking changes.")
	}

	if analysis.IsSecurityRelevant(a.Category, a.Message) {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  NOTE: This commit is security-relevant.")
	}
}

func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		if line == "" {
			result[i] = ""
		} else {
			result[i] = prefix + line
		}
	}
	return strings.Join(result, "\n")
}

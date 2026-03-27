package sync

import (
	"bytes"
	"context"
	"strings"
	"text/template"

	"github.com/0xmhha/commitflow/internal/git"
)

// ApplicabilityData holds all context needed to construct an applicability prompt.
type ApplicabilityData struct {
	UpstreamHash     string
	Message          string
	Category         string
	ImpactScore      int
	Packages         []string
	Diff             string
	DivergencePoint  string
	ForkOnlyPackages []string
	OverlappingPkgs  []string
	MissingPkgs      []string
	StatSummary      string
}

// ForkContext contains information about the fork repository gathered
// relative to an upstream commit's changed files.
type ForkContext struct {
	OverlappingFiles    []string
	NonOverlappingFiles []string
	ForkOnlyDirs        []string
}

const applicabilityPromptTemplate = `You are evaluating whether an upstream commit should be applied to a fork repository.

UPSTREAM COMMIT:
- Hash: {{.UpstreamHash}}
- Message: {{.Message}}
- Category: {{.Category}}
- Impact Score: {{.ImpactScore}}/10
{{- if .Packages}}
- Packages affected: {{join .Packages ", "}}
{{- end}}

FORK DIVERGENCE:
- Fork diverged at: {{.DivergencePoint}}
{{- if .ForkOnlyPackages}}
- Fork-only packages (not in upstream): {{join .ForkOnlyPackages ", "}}
{{- end}}
{{- if .OverlappingPkgs}}
- Overlapping packages (exist in both): {{join .OverlappingPkgs ", "}}
{{- end}}
{{- if .MissingPkgs}}
- Files changed in upstream but absent in fork: {{join .MissingPkgs ", "}}
{{- end}}
{{- if .StatSummary}}

DIFF SUMMARY:
{{.StatSummary}}
{{- end}}
{{- if .Diff}}

DIFF:
{{.Diff}}
{{- end}}

Assess whether this upstream commit is applicable to the fork. Consider:
1. Does the fork contain the files/packages affected by this commit?
2. Is the change relevant to the fork's purpose given fork-only customisations?
3. How likely is a merge conflict given the fork's divergence?
4. What is the recommended action (apply, skip, manual review)?`

var compiledApplicabilityTemplate = template.Must(
	template.New("applicability").
		Funcs(template.FuncMap{"join": strings.Join}).
		Parse(applicabilityPromptTemplate),
)

// BuildApplicabilityPrompt renders the applicability assessment prompt.
func BuildApplicabilityPrompt(data ApplicabilityData) (string, error) {
	var buf bytes.Buffer
	if err := compiledApplicabilityTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// CheckFileOverlap checks which files changed in the upstream commit also
// exist in the fork repository. Returns (overlapping, nonOverlapping).
func CheckFileOverlap(ctx context.Context, forkRepo *git.Repository, filesChanged []string) ([]string, []string) {
	overlapping := make([]string, 0, len(filesChanged))
	nonOverlapping := make([]string, 0)

	for _, f := range filesChanged {
		if forkRepo.FileExists(ctx, f) {
			overlapping = append(overlapping, f)
		} else {
			nonOverlapping = append(nonOverlapping, f)
		}
	}

	return overlapping, nonOverlapping
}

// GatherForkContext collects information about the fork repository relative
// to the set of files changed in an upstream commit.
func GatherForkContext(ctx context.Context, forkRepo *git.Repository, filesChanged []string) (*ForkContext, error) {
	overlapping, nonOverlapping := CheckFileOverlap(ctx, forkRepo, filesChanged)

	forkOnlyDirs, err := listForkOnlyDirs(ctx, forkRepo, filesChanged)
	if err != nil {
		return nil, err
	}

	return &ForkContext{
		OverlappingFiles:    overlapping,
		NonOverlappingFiles: nonOverlapping,
		ForkOnlyDirs:        forkOnlyDirs,
	}, nil
}

// listForkOnlyDirs returns top-level directories present in the fork that are
// not represented in the upstream's changed file list. This indicates
// fork-specific additions that may be unrelated to upstream changes.
func listForkOnlyDirs(ctx context.Context, forkRepo *git.Repository, filesChanged []string) ([]string, error) {
	upstreamDirs := make(map[string]struct{}, len(filesChanged))
	for _, f := range filesChanged {
		dir := topLevelDir(f)
		if dir != "" {
			upstreamDirs[dir] = struct{}{}
		}
	}

	hashes, err := forkRepo.ListCommits(ctx, git.CommitListOpts{Last: 1})
	if err != nil {
		return nil, err
	}
	if len(hashes) == 0 {
		return nil, nil
	}

	diff, err := forkRepo.GetCommitDiff(ctx, hashes[0], git.DiffOpts{})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	forkOnlyDirs := make([]string, 0)

	for _, fd := range diff.FileDiffs {
		dir := topLevelDir(fd.Path)
		if dir == "" {
			continue
		}
		if _, isUpstream := upstreamDirs[dir]; isUpstream {
			continue
		}
		if _, already := seen[dir]; already {
			continue
		}
		seen[dir] = struct{}{}
		forkOnlyDirs = append(forkOnlyDirs, dir)
	}

	return forkOnlyDirs, nil
}

// topLevelDir extracts the first path component from a file path.
// Returns an empty string for files at the root level.
func topLevelDir(filePath string) string {
	idx := strings.IndexByte(filePath, '/')
	if idx <= 0 {
		return ""
	}
	return filePath[:idx]
}

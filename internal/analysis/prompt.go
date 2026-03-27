package analysis

import (
	"bytes"
	"text/template"
)

// PromptData holds all the data needed to fill a commit analysis prompt template.
type PromptData struct {
	Hash           string
	Author         string
	AuthorEmail    string
	Date           string
	Message        string
	FilesChanged   []string
	Additions      int
	Deletions      int
	Diff           string
	DiffTruncated  bool
	TruncationNote string
	TotalDiffLines int
	StatSummary    string
}

const analysisPromptTemplate = `You are analyzing a git commit. Analyze the changes and classify them.

COMMIT METADATA:
- Hash: {{.Hash}}
- Author: {{.Author}} <{{.AuthorEmail}}>
- Date: {{.Date}}
- Message: {{.Message}}
- Files changed: {{len .FilesChanged}}
- Additions: {{.Additions}}, Deletions: {{.Deletions}}
{{if .DiffTruncated}}
NOTE: {{.TruncationNote}}
FILE SUMMARY:
{{.StatSummary}}
{{end}}
{{if .Diff}}
DIFF:
{{.Diff}}
{{end}}
Analyze this commit and classify it. Consider:
1. What category best describes this change?
2. What is the impact on the codebase (1=trivial, 10=critical)?
3. Does this introduce breaking changes to public APIs?
4. Which packages/directories are affected?
5. Is this a security-relevant change?`

var compiledTemplate = template.Must(template.New("analysis").Parse(analysisPromptTemplate))

// BuildAnalysisPrompt constructs the prompt for commit analysis using the
// standard template. Returns the rendered prompt string or a build error.
func BuildAnalysisPrompt(data PromptData) (string, error) {
	var buf bytes.Buffer
	if err := compiledTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

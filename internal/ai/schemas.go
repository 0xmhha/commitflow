package ai

import "fmt"

// CommitAnalysisOutput is the Go type matching the commit analysis JSON schema.
type CommitAnalysisOutput struct {
	Category              string   `json:"category"`
	Summary               string   `json:"summary"`
	ImpactScore           int      `json:"impact_score"`
	BreakingChanges       bool     `json:"breaking_changes"`
	BreakingChangeDetails string   `json:"breaking_change_details,omitempty"`
	PackagesAffected      []string `json:"packages_affected"`
	SecurityRelevant      bool     `json:"security_relevant"`
	SecurityDetails       string   `json:"security_details,omitempty"`
	DetailedAnalysis      string   `json:"detailed_analysis,omitempty"`
}

// ApplicabilityOutput is the Go type matching the applicability assessment JSON schema.
type ApplicabilityOutput struct {
	Status               string   `json:"status"`
	Reason               string   `json:"reason"`
	RelevanceScore       int      `json:"relevance_score"`
	ConflictLikelihood   string   `json:"conflict_likelihood"`
	RecommendedAction    string   `json:"recommended_action"`
	AffectedForkPackages []string `json:"affected_fork_packages,omitempty"`
}

// CommitAnalysisSchema is the JSON schema string for commit analysis structured output.
// Matches ARCHITECTURE.md section 3.3 exactly.
const CommitAnalysisSchema = `{
  "type": "object",
  "properties": {
    "category": {
      "type": "string",
      "enum": ["bug_fix", "feature", "refactor", "security",
               "performance", "docs", "chore", "breaking_change"]
    },
    "summary": {
      "type": "string",
      "description": "커밋의 목적과 변경 내용을 설명하는 한 문단 요약"
    },
    "impact_score": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10,
      "description": "코드베이스에 미치는 영향도 (1=사소, 10=크리티컬)"
    },
    "breaking_changes": { "type": "boolean" },
    "breaking_change_details": { "type": "string" },
    "packages_affected": {
      "type": "array",
      "items": { "type": "string" }
    },
    "security_relevant": { "type": "boolean" },
    "security_details": { "type": "string" },
    "detailed_analysis": {
      "type": "string",
      "description": "변경사항의 기술적 분석 (다중 문단)"
    }
  },
  "required": ["category", "summary", "impact_score",
               "breaking_changes", "packages_affected", "security_relevant"]
}`

// ApplicabilitySchema is the JSON schema string for applicability assessment.
// Matches ARCHITECTURE.md section 3.4 exactly.
const ApplicabilitySchema = `{
  "type": "object",
  "properties": {
    "status": {
      "type": "string",
      "enum": ["applicable", "not_applicable", "needs_review", "already_applied"]
    },
    "reason": {
      "type": "string",
      "description": "판단 근거 설명"
    },
    "relevance_score": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10
    },
    "conflict_likelihood": {
      "type": "string",
      "enum": ["none", "low", "medium", "high"]
    },
    "recommended_action": {
      "type": "string",
      "description": "권장 조치 설명"
    },
    "affected_fork_packages": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "required": ["status", "reason", "relevance_score",
               "conflict_likelihood", "recommended_action"]
}`

// Valid category values for CommitAnalysisOutput.Category.
const (
	CategoryBugFix         = "bug_fix"
	CategoryFeature        = "feature"
	CategoryRefactor       = "refactor"
	CategorySecurity       = "security"
	CategoryPerformance    = "performance"
	CategoryDocs           = "docs"
	CategoryChore          = "chore"
	CategoryBreakingChange = "breaking_change"
)

// Valid applicability statuses for ApplicabilityOutput.Status.
const (
	StatusApplicable     = "applicable"
	StatusNotApplicable  = "not_applicable"
	StatusNeedsReview    = "needs_review"
	StatusAlreadyApplied = "already_applied"
)

// Valid conflict likelihood values for ApplicabilityOutput.ConflictLikelihood.
const (
	ConflictNone   = "none"
	ConflictLow    = "low"
	ConflictMedium = "medium"
	ConflictHigh   = "high"
)

// ValidCategories returns all valid commit category values.
func ValidCategories() []string {
	return []string{
		CategoryBugFix,
		CategoryFeature,
		CategoryRefactor,
		CategorySecurity,
		CategoryPerformance,
		CategoryDocs,
		CategoryChore,
		CategoryBreakingChange,
	}
}

// ValidStatuses returns all valid applicability status values.
func ValidStatuses() []string {
	return []string{
		StatusApplicable,
		StatusNotApplicable,
		StatusNeedsReview,
		StatusAlreadyApplied,
	}
}

// ValidConflictLikelihoods returns all valid conflict likelihood values.
func ValidConflictLikelihoods() []string {
	return []string{
		ConflictNone,
		ConflictLow,
		ConflictMedium,
		ConflictHigh,
	}
}

const maxStringLen = 10000

// ValidateCommitAnalysis checks that AI-generated commit analysis output
// contains valid enum values and reasonable field ranges.
func ValidateCommitAnalysis(o *CommitAnalysisOutput) error {
	if !isValidValue(o.Category, ValidCategories()) {
		return fmt.Errorf("invalid category: %q", o.Category)
	}
	if o.ImpactScore < 1 || o.ImpactScore > 10 {
		return fmt.Errorf("impact_score must be 1-10, got %d", o.ImpactScore)
	}
	if len(o.Summary) > maxStringLen {
		o.Summary = o.Summary[:maxStringLen]
	}
	if len(o.DetailedAnalysis) > maxStringLen {
		o.DetailedAnalysis = o.DetailedAnalysis[:maxStringLen]
	}
	return nil
}

// ValidateApplicability checks that AI-generated applicability output
// contains valid enum values and reasonable field ranges.
func ValidateApplicability(o *ApplicabilityOutput) error {
	if !isValidValue(o.Status, ValidStatuses()) {
		return fmt.Errorf("invalid status: %q", o.Status)
	}
	if o.RelevanceScore < 1 || o.RelevanceScore > 10 {
		return fmt.Errorf("relevance_score must be 1-10, got %d", o.RelevanceScore)
	}
	if !isValidValue(o.ConflictLikelihood, ValidConflictLikelihoods()) {
		return fmt.Errorf("invalid conflict_likelihood: %q", o.ConflictLikelihood)
	}
	return nil
}

func isValidValue(val string, valid []string) bool {
	for _, v := range valid {
		if val == v {
			return true
		}
	}
	return false
}

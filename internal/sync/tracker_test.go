package sync

import (
	"testing"
)

func TestUpdateScanResultCounts(t *testing.T) {
	tests := []struct {
		status         string
		wantApplicable int
		wantNotApp     int
		wantReview     int
		wantApplied    int
	}{
		{"applicable", 1, 0, 0, 0},
		{"not_applicable", 0, 1, 0, 0},
		{"needs_review", 0, 0, 1, 0},
		{"already_applied", 0, 0, 0, 1},
		{"unknown_status", 0, 0, 0, 0},
	}

	for _, tt := range tests {
		result := &ScanResult{}
		updateScanResultCounts(result, tt.status, 1.23)

		if result.Applicable != tt.wantApplicable {
			t.Errorf("status=%q: Applicable = %d, want %d", tt.status, result.Applicable, tt.wantApplicable)
		}
		if result.NotApplicable != tt.wantNotApp {
			t.Errorf("status=%q: NotApplicable = %d, want %d", tt.status, result.NotApplicable, tt.wantNotApp)
		}
		if result.NeedsReview != tt.wantReview {
			t.Errorf("status=%q: NeedsReview = %d, want %d", tt.status, result.NeedsReview, tt.wantReview)
		}
		if result.AlreadyApplied != tt.wantApplied {
			t.Errorf("status=%q: AlreadyApplied = %d, want %d", tt.status, result.AlreadyApplied, tt.wantApplied)
		}
		if result.TotalCost != 1.23 {
			t.Errorf("status=%q: TotalCost = %f, want 1.23", tt.status, result.TotalCost)
		}
	}
}

func TestCollectFilePaths_SyncPackage(t *testing.T) {
	// Import git to test with real FileDiff type.
	// This is tested via the git package but we ensure it compiles here.
	paths := collectFilePaths(nil)
	if len(paths) != 0 {
		t.Errorf("collectFilePaths(nil) returned %d paths, want 0", len(paths))
	}
}

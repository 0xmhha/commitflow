package ai

import (
	"encoding/json"
	"testing"
)

func TestParseResponse_ValidJSON(t *testing.T) {
	data := `{
		"type": "result",
		"is_error": false,
		"duration_ms": 1234,
		"result": "ok",
		"total_cost_usd": 0.05,
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"cache_creation_input_tokens": 10,
			"cache_read_input_tokens": 5
		},
		"structured_output": {"category": "bug_fix"}
	}`

	resp, err := ParseResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Type != "result" {
		t.Errorf("Type = %q, want %q", resp.Type, "result")
	}
	if resp.IsError {
		t.Error("IsError = true, want false")
	}
	if resp.DurationMS != 1234 {
		t.Errorf("DurationMS = %d, want 1234", resp.DurationMS)
	}
	if resp.TotalCostUSD != 0.05 {
		t.Errorf("TotalCostUSD = %f, want 0.05", resp.TotalCostUSD)
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", resp.Usage.OutputTokens)
	}
	if resp.Usage.CacheCreationInputTokens != 10 {
		t.Errorf("CacheCreationInputTokens = %d, want 10", resp.Usage.CacheCreationInputTokens)
	}
}

func TestParseResponse_EmptyData(t *testing.T) {
	_, err := ParseResponse([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	_, err := ParseResponse([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseResponse_IsErrorTrue(t *testing.T) {
	data := `{"type":"result","is_error":true,"result":"something went wrong"}`
	_, err := ParseResponse([]byte(data))
	if err == nil {
		t.Fatal("expected error when is_error=true, got nil")
	}
}

func TestExtractStructuredOutput(t *testing.T) {
	raw := json.RawMessage(`{"category":"bug_fix","summary":"fixed a bug","impact_score":5,"breaking_changes":false,"packages_affected":["pkg/a"],"security_relevant":false}`)
	resp := &Response{
		StructuredOutput: raw,
	}

	output, err := ExtractStructuredOutput[CommitAnalysisOutput](resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Category != "bug_fix" {
		t.Errorf("Category = %q, want %q", output.Category, "bug_fix")
	}
	if output.Summary != "fixed a bug" {
		t.Errorf("Summary = %q, want %q", output.Summary, "fixed a bug")
	}
	if output.ImpactScore != 5 {
		t.Errorf("ImpactScore = %d, want 5", output.ImpactScore)
	}
	if output.BreakingChanges {
		t.Error("BreakingChanges = true, want false")
	}
}

func TestExtractStructuredOutput_NilResponse(t *testing.T) {
	_, err := ExtractStructuredOutput[CommitAnalysisOutput](nil)
	if err == nil {
		t.Fatal("expected error for nil response, got nil")
	}
}

func TestExtractStructuredOutput_EmptyStructuredOutput(t *testing.T) {
	resp := &Response{}
	_, err := ExtractStructuredOutput[CommitAnalysisOutput](resp)
	if err == nil {
		t.Fatal("expected error for empty structured_output, got nil")
	}
}

func TestToCallLog(t *testing.T) {
	resp := &Response{
		DurationMS:   500,
		TotalCostUSD: 0.02,
		Usage: Usage{
			InputTokens:  200,
			OutputTokens: 80,
		},
	}

	log := resp.ToCallLog("analysis", "abc123", "sonnet")
	if log.CallType != "analysis" {
		t.Errorf("CallType = %q, want %q", log.CallType, "analysis")
	}
	if log.CommitHash != "abc123" {
		t.Errorf("CommitHash = %q, want %q", log.CommitHash, "abc123")
	}
	if log.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", log.Model, "sonnet")
	}
	if log.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200", log.InputTokens)
	}
	if log.CostUSD != 0.02 {
		t.Errorf("CostUSD = %f, want 0.02", log.CostUSD)
	}
}

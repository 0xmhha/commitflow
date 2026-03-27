package ai

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Response represents the JSON output from Claude CLI.
type Response struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"`
	IsError          bool            `json:"is_error"`
	DurationMS       int             `json:"duration_ms"`
	DurationAPIMS    int             `json:"duration_api_ms"`
	NumTurns         int             `json:"num_turns"`
	Result           string          `json:"result"`
	StopReason       string          `json:"stop_reason"`
	SessionID        string          `json:"session_id"`
	TotalCostUSD     float64         `json:"total_cost_usd"`
	Usage            Usage           `json:"usage"`
	StructuredOutput json.RawMessage `json:"structured_output"`
}

// Usage tracks token consumption for a Claude CLI call.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// CallLog represents a record of an AI call for cost tracking.
type CallLog struct {
	CallType     string
	CommitHash   string
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	DurationMS   int
}

// ParseResponse parses the raw JSON output from Claude CLI.
func ParseResponse(data []byte) (*Response, error) {
	if len(data) == 0 {
		return nil, errors.New("parse response: empty response data")
	}

	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse response: invalid JSON: %w", err)
	}

	if resp.IsError {
		return nil, fmt.Errorf("claude returned error: %s", resp.Result)
	}

	return &resp, nil
}

// ExtractStructuredOutput unmarshals the structured_output field into the target type T.
func ExtractStructuredOutput[T any](resp *Response) (*T, error) {
	if resp == nil {
		return nil, errors.New("extract structured output: nil response")
	}

	if len(resp.StructuredOutput) == 0 {
		return nil, errors.New("extract structured output: structured_output field is empty")
	}

	var target T
	if err := json.Unmarshal(resp.StructuredOutput, &target); err != nil {
		return nil, fmt.Errorf("extract structured output: %w", err)
	}

	return &target, nil
}

// ToCallLog converts a Response to a CallLog for cost tracking.
func (r *Response) ToCallLog(callType, commitHash, model string) CallLog {
	return CallLog{
		CallType:     callType,
		CommitHash:   commitHash,
		Model:        model,
		InputTokens:  r.Usage.InputTokens,
		OutputTokens: r.Usage.OutputTokens,
		CostUSD:      r.TotalCostUSD,
		DurationMS:   r.DurationMS,
	}
}

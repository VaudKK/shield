package ai

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		result  AnalysisResult
		wantErr bool
	}{
		{
			name: "valid result",
			result: AnalysisResult{
				Summary:  "ok",
				Timeline: []TimelineEntry{{Date: "unknown", Description: "something happened"}},
				PII:      []PIIEntry{{Type: "email", Value: "a@b.com", Location: "..."}},
				Gaps:     []GapEntry{{Description: "missing date", Confidence: "medium"}},
			},
			wantErr: false,
		},
		{
			name: "nil arrays rejected",
			result: AnalysisResult{
				Summary: "ok",
			},
			wantErr: true,
		},
		{
			name: "invalid confidence rejected",
			result: AnalysisResult{
				Timeline: []TimelineEntry{},
				PII:      []PIIEntry{},
				Gaps:     []GapEntry{{Description: "x", Confidence: "extremely-high"}},
			},
			wantErr: true,
		},
		{
			name: "timeline entry missing description rejected",
			result: AnalysisResult{
				Timeline: []TimelineEntry{{Date: "unknown", Description: ""}},
				PII:      []PIIEntry{},
				Gaps:     []GapEntry{},
			},
			wantErr: true,
		},
		{
			name: "pii entry missing value rejected",
			result: AnalysisResult{
				Timeline: []TimelineEntry{},
				PII:      []PIIEntry{{Type: "email", Value: ""}},
				Gaps:     []GapEntry{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

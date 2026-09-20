package ai

// analysisSchema is a strict JSON Schema (OpenAI Structured Outputs
// requires every object to list all its properties as required and set
// additionalProperties: false) matching AnalysisResult exactly.
var analysisSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"summary", "timeline", "pii", "gaps"},
	"properties": map[string]any{
		"summary": map[string]any{
			"type": "string",
			"description": "A neutral, plain-language summary of what the evidence appears to " +
				"show, grounded only in the provided text. Use hedged language such as " +
				"'the document appears to state' or 'the available evidence contains' — " +
				"never assert that something definitely happened or assign fault.",
		},
		"timeline": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"date", "description"},
				"properties": map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "Date in YYYY-MM-DD format if it can be determined from the text, otherwise the literal string \"unknown\". Never guess.",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "What happened at this point, grounded in the text.",
					},
				},
			},
		},
		"pii": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"type", "value", "location"},
				"properties": map[string]any{
					"type": map[string]any{
						"type":        "string",
						"description": "e.g. name, phone_number, email, address, national_id, vehicle_registration, date_of_birth",
					},
					"value": map[string]any{"type": "string"},
					"location": map[string]any{
						"type":        "string",
						"description": "A short snippet of surrounding text showing where this was found.",
					},
				},
			},
		},
		"gaps": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"description", "confidence"},
				"properties": map[string]any{
					"description": map[string]any{
						"type":        "string",
						"description": "Missing or unclear information — never a suggestion of what the user should claim.",
					},
					"confidence": map[string]any{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
				},
			},
		},
	},
}

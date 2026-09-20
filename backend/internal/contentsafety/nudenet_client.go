package contentsafety

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type NudeNetClient struct {
	baseURL string
	http    *http.Client
}

func NewNudeNetClient(baseURL string) *NudeNetClient {
	return &NudeNetClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

type classifyResponse struct {
	Labels []struct {
		Label string  `json:"label"`
		Score float64 `json:"score"`
	} `json:"labels"`
	Sensitive         bool    `json:"sensitive"`
	MaxSensitiveScore float64 `json:"max_sensitive_score"`
}

func (c *NudeNetClient) Classify(ctx context.Context, imageBytes []byte, filename string) (*Classification, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create multipart field: %w", err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		return nil, fmt.Errorf("write image bytes: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/classify", &body)
	if err != nil {
		return nil, fmt.Errorf("build classify request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call content safety service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("content safety service returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed classifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode classify response: %w", err)
	}

	labels := make([]Label, len(parsed.Labels))
	for i, l := range parsed.Labels {
		labels[i] = Label{Name: l.Label, Score: l.Score}
	}

	return &Classification{
		Labels:            labels,
		Sensitive:         parsed.Sensitive,
		MaxSensitiveScore: parsed.MaxSensitiveScore,
	}, nil
}

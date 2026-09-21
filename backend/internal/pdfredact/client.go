package pdfredact

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

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Redact(ctx context.Context, pdfBytes []byte, values []string) (*Result, error) {
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal values: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "document.pdf")
	if err != nil {
		return nil, fmt.Errorf("create multipart field: %w", err)
	}
	if _, err := part.Write(pdfBytes); err != nil {
		return nil, fmt.Errorf("write pdf bytes: %w", err)
	}
	if err := writer.WriteField("values", string(valuesJSON)); err != nil {
		return nil, fmt.Errorf("write values field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/redact", &body)
	if err != nil {
		return nil, fmt.Errorf("build redact request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call pdf redaction service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("pdf redaction service returned %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read redact response: %w", err)
	}
	return decodeRedactResponse(respBody)
}

package tools

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/sirupsen/logrus"
)

// NewAnalyzeRemoteResourceTool creates a tool for analyzing remote URLs (like PDFs or documents) provided by the user.
func NewAnalyzeRemoteResourceTool(instanceID string) ToolDefinition {
	return ToolDefinition{
		Tool: domainMCP.Tool{
			Name:        "analyze_remote_resource",
			Description: "Call this tool when the user provides a remote URL (HTTP/HTTPS link) to a document (PDF, DOCX, XLSX, etc.) or an image that they explicitly want you to analyze. This tool will validate the URL and forward it to your multimodal vision engine to extract the content.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "The exact URL of the document or media to analyze (must be http or https).",
					},
					"intent": map[string]any{
						"type":        "string",
						"description": "The goal of the analysis. Example: 'extract text', 'summarize this document', 'describe the image'.",
					},
				},
				"required": []string{"url", "intent"},
			},
		},
		Handler: func(ctx context.Context, contextData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			rawURL, ok := args["url"].(string)
			if !ok || rawURL == "" {
				return nil, fmt.Errorf("url is required")
			}
			intent, _ := args["intent"].(string)
			if intent == "" {
				intent = "analyze this remote file"
			}

			// Validate URL
			parsedURL, err := url.ParseRequestURI(rawURL)
			if err != nil {
				return nil, fmt.Errorf("invalid URL format: %v", err)
			}
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				return nil, fmt.Errorf("URL scheme must be http or https")
			}

			// Extract filename from URL path if possible
			filename := path.Base(parsedURL.Path)
			if filename == "" || filename == "/" || filename == "." {
				filename = "remote_file.bin"
			}

			// Check headers with HEAD request
			customTransport := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			httpClient := &http.Client{Transport: customTransport, Timeout: 10 * time.Second}

			req, err := http.NewRequest("HEAD", rawURL, nil)
			if err != nil {
				return nil, fmt.Errorf("could not create request: %v", err)
			}

			// Add a common User-Agent to avoid early blocks
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

			resp, err := httpClient.Do(req)
			mimeType := "application/octet-stream"

			if err == nil && resp != nil {
				defer resp.Body.Close()
				contentType := resp.Header.Get("Content-Type")
				if contentType != "" {
					mimeType = contentType
				}

				// Check size to avoid massive files (e.g. limit 50MB)
				if resp.ContentLength > 50*1024*1024 {
					return nil, fmt.Errorf("remote file is too large (%.2f MB), maximum allowed is 50MB", float64(resp.ContentLength)/(1024*1024))
				}
			} else {
				logrus.WithError(err).Warnf("[ANALYZE_URL] HEAD request failed for %s, will attempt directly", rawURL)
			}

			// Delegate to orchestrator marking is_url=true
			return map[string]interface{}{
				"action":    "trigger_multimodal_analysis",
				"path":      rawURL,
				"mime_type": mimeType,
				"filename":  filename,
				"intent":    intent,
				"is_url":    true, // flag to inform orchestrator
				"status":    "analysis_triggered",
				"message":   fmt.Sprintf("Sent remote file %s to multimodal engine with intent: '%s'", filename, intent),
			}, nil
		},
	}
}

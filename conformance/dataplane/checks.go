package dataplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/report"
)

func check01Health(ctx context.Context, base, secret string) report.Result {
	return timedCheck(1, "GET /v1/health", func() (report.Status, string) {
		status, body, err := httpGet(ctx, base+"/v1/health", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected 200, got %d", status)
		}
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			return report.Fail, fmt.Sprintf("invalid JSON: %v", err)
		}
		if _, ok := data["status"]; !ok {
			return report.Fail, "missing field: status"
		}
		return report.Pass, ""
	})
}

func check02Readyz(ctx context.Context, base, secret string) report.Result {
	return timedCheck(2, "GET /v1/readyz", func() (report.Status, string) {
		status, body, err := httpGet(ctx, base+"/v1/readyz", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 && status != 503 {
			return report.Fail, fmt.Sprintf("expected 200 or 503, got %d", status)
		}
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			return report.Fail, fmt.Sprintf("invalid JSON: %v", err)
		}
		if _, ok := data["status"]; !ok {
			return report.Fail, "missing field: status"
		}
		return report.Pass, ""
	})
}

func check03AdminAuthRequired(ctx context.Context, base string) report.Result {
	return timedCheck(3, "Admin endpoints require auth", func() (report.Status, string) {
		status, _, err := httpGet(ctx, base+"/v1/admin/sources", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status == 200 {
			return report.Fail, "expected 401, got 200 (admin endpoint is ungated)"
		}
		if status != 401 {
			return report.Fail, fmt.Sprintf("expected 401, got %d", status)
		}
		return report.Pass, ""
	})
}

func check04AdminInvalidAuth(ctx context.Context, base string) report.Result {
	return timedCheck(4, "Admin: invalid bearer rejected", func() (report.Status, string) {
		status, _, err := httpGet(ctx, base+"/v1/admin/sources", map[string]string{
			"Authorization": "Bearer wrong-secret",
		})
		if err != nil {
			return report.Fail, err.Error()
		}
		if status == 200 {
			return report.Fail, "expected 401, got 200 (invalid bearer was accepted)"
		}
		return report.Pass, ""
	})
}

func check05AdminListSources(ctx context.Context, base, secret string) report.Result {
	return timedCheck(5, "Admin: list sources", func() (report.Status, string) {
		status, body, err := httpGet(ctx, base+"/v1/admin/sources", authHeader(secret))
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected 200, got %d", status)
		}
		var list []any
		if err := json.Unmarshal(body, &list); err != nil {
			return report.Fail, fmt.Sprintf("expected JSON array: %v", err)
		}
		return report.Pass, ""
	})
}

func check06SourceCRUD(ctx context.Context, base, secret string) report.Result {
	return timedCheck(6, "Source create-get-delete round trip", func() (report.Status, string) {
		createBody := `{"id":"conf-test-src","type":"rest","name":"Conformance Test","collection":"conf-test","config":{"url":"http://127.0.0.1:0/noop"}}`
		status, _, err := httpPost(ctx, base+"/v1/admin/sources", createBody, authHeader(secret))
		if err != nil {
			return report.Fail, fmt.Sprintf("create: %v", err)
		}
		if status != 201 {
			return report.Fail, fmt.Sprintf("create: expected 201, got %d", status)
		}

		status, body, err := httpGet(ctx, base+"/v1/admin/sources/conf-test-src", authHeader(secret))
		if err != nil {
			return report.Fail, fmt.Sprintf("get: %v", err)
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("get: expected 200, got %d", status)
		}
		var src map[string]any
		if err := json.Unmarshal(body, &src); err != nil {
			return report.Fail, fmt.Sprintf("get: invalid JSON: %v", err)
		}
		if src["id"] != "conf-test-src" {
			return report.Fail, fmt.Sprintf("get: id = %v, want conf-test-src", src["id"])
		}

		status, _, err = httpDelete(ctx, base+"/v1/admin/sources/conf-test-src", authHeader(secret))
		if err != nil {
			return report.Fail, fmt.Sprintf("delete: %v", err)
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("delete: expected 200, got %d", status)
		}

		status, _, err = httpGet(ctx, base+"/v1/admin/sources/conf-test-src", authHeader(secret))
		if err != nil {
			return report.Fail, fmt.Sprintf("verify delete: %v", err)
		}
		if status != 404 {
			return report.Fail, fmt.Sprintf("verify delete: expected 404, got %d", status)
		}

		return report.Pass, ""
	})
}

func check07QueryAuthRequired(ctx context.Context, base string) report.Result {
	return timedCheck(7, "Query endpoint requires auth", func() (report.Status, string) {
		body := `{"query":"test"}`
		status, _, err := httpPost(ctx, base+"/v1/query", body, nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status == 200 {
			return report.Fail, "expected 401, got 200 (query endpoint is ungated)"
		}
		if status != 401 {
			return report.Fail, fmt.Sprintf("expected 401, got %d", status)
		}
		return report.Pass, ""
	})
}

func check08QueryWithAuth(ctx context.Context, base, secret string) report.Result {
	return timedCheck(8, "Query with valid auth returns results", func() (report.Status, string) {
		body := `{"query":"conformance probe"}`
		status, respBody, err := httpPost(ctx, base+"/v1/query", body, authHeader(secret))
		if err != nil {
			return report.Fail, err.Error()
		}
		if status == 503 {
			return report.Skip, "embedding/vector service unavailable"
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected 200, got %d: %s", status, string(respBody))
		}
		var data map[string]any
		if err := json.Unmarshal(respBody, &data); err != nil {
			return report.Fail, fmt.Sprintf("invalid JSON: %v", err)
		}
		if _, ok := data["results"]; !ok {
			return report.Fail, "missing field: results"
		}
		return report.Pass, ""
	})
}

func check09SourceListConsistency(ctx context.Context, base, secret string) report.Result {
	return timedCheck(9, "Source list returns JSON array", func() (report.Status, string) {
		status, body, err := httpGet(ctx, base+"/v1/sources", map[string]string{
			"Authorization": "Bearer " + secret,
		})
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected 200, got %d", status)
		}
		var list []any
		if err := json.Unmarshal(body, &list); err != nil {
			return report.Fail, fmt.Sprintf("expected JSON array: %v", err)
		}
		return report.Pass, ""
	})
}

func check10HealthContentType(ctx context.Context, base string) report.Result {
	return timedCheck(10, "Health returns application/json", func() (report.Status, string) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/health", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		c := &http.Client{Timeout: 10 * time.Second}
		resp, err := c.Do(req)
		if err != nil {
			return report.Fail, err.Error()
		}
		resp.Body.Close()
		ct := resp.Header.Get("Content-Type")
		if ct != "application/json" {
			return report.Fail, fmt.Sprintf("Content-Type = %q, want application/json", ct)
		}
		return report.Pass, ""
	})
}

// --- helpers ---

func timedCheck(num int, name string, fn func() (report.Status, string)) report.Result {
	start := time.Now()
	status, detail := fn()
	return report.Result{
		Number:   num,
		Name:     name,
		Status:   status,
		Duration: time.Since(start),
		Detail:   detail,
	}
}

func authHeader(secret string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + secret}
}

func httpGet(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

func httpPost(ctx context.Context, url, jsonBody string, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(jsonBody)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

func httpDelete(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

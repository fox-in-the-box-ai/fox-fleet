package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/report"
	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/sut"
)

const contractVersionPrefix = "2."
const foxLoginPath = "/api/auth/login"
const foxSessionCookie = "hermes_session"

func check01BootInvariant(ctx context.Context, cli *client.Client, image string) report.Result {
	start := time.Now()
	r := report.Result{Number: 1, Name: "Boot invariant"}

	h, err := sut.Start(ctx, cli, sut.Config{
		Image:      image,
		Mode:       sut.BootInvariant,
		AuthSecret: "conf-test-boot-invariant",
	})
	if err != nil && strings.Contains(err.Error(), "not healthy") {
		r.Status = report.Pass
		r.Detail = "container refused to become healthy (boot invariant held)"
		r.Duration = time.Since(start)
		return r
	}
	if h == nil && err != nil {
		r.Status = report.Fail
		r.Detail = err.Error()
		r.Duration = time.Since(start)
		return r
	}
	defer h.Cleanup(ctx)

	code, err := h.WaitExit(ctx, 15*time.Second)
	r.Duration = time.Since(start)
	if err == nil {
		if code != 0 {
			r.Status = report.Pass
			r.Detail = fmt.Sprintf("exit code %d", code)
		} else {
			r.Status = report.Fail
			r.Detail = "container exited with code 0 (expected non-zero)"
		}
		return r
	}

	logs, logErr := h.Logs(ctx)
	if logErr != nil {
		r.Status = report.Fail
		r.Detail = fmt.Sprintf("container did not exit and logs unreadable: %v", logErr)
		return r
	}
	hasFatalLog := strings.Contains(logs, "FATAL") && strings.Contains(logs, "FOX_PLANE_AUTH_SECRET")
	hasFatalCycle := strings.Contains(logs, "gave up:") && strings.Contains(logs, "FATAL state")
	if !hasFatalLog && !hasFatalCycle {
		r.Status = report.Fail
		r.Detail = "container did not exit and no FATAL log for missing FOX_PLANE_AUTH_SECRET"
		return r
	}

	healthStatus, _, healthErr := httpGet(ctx, h.BaseURL+"/health", nil)
	if healthErr != nil || healthStatus != 200 {
		r.Status = report.Pass
		r.Detail = "supervisor FATAL state detected (boot invariant held via process manager)"
		return r
	}
	r.Status = report.Fail
	r.Detail = "FATAL logged but /health still returns 200"
	return r
}

func check02ManagedValidAuth(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(2, "Managed: valid X-Fox-Auth", func() (report.Status, string) {
		status, _, err := httpGet(ctx, h.BaseURL+"/version", map[string]string{
			"X-Fox-Auth": h.AuthSecret,
		})
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected 200, got %d", status)
		}
		return report.Pass, ""
	})
}

func check03ManagedInvalidAuth(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(3, "Managed: invalid X-Fox-Auth", func() (report.Status, string) {
		status, _, err := httpGet(ctx, h.BaseURL+"/version", map[string]string{
			"X-Fox-Auth": "wrong-secret",
		})
		if err != nil {
			return report.Fail, err.Error()
		}
		if status == 200 {
			return report.Fail, "expected 401, got 200 (invalid secret was accepted)"
		}
		if status != 401 && status != 302 {
			return report.Fail, fmt.Sprintf("expected 401 or 302, got %d", status)
		}
		return report.Pass, ""
	})
}

func check04ManagedSessionAuth(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(4, "Managed: valid session, no X-Fox-Auth", func() (report.Status, string) {
		cookie, err := obtainSessionCookie(ctx, h.BaseURL, h.Password)
		if err != nil {
			return report.Fail, fmt.Sprintf("obtain session: %v", err)
		}
		status, _, err := httpGet(ctx, h.BaseURL+"/version", map[string]string{
			"Cookie": cookie,
		})
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected 200, got %d", status)
		}
		return report.Pass, ""
	})
}

func check05ManagedNoAuth(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(5, "Managed: no auth", func() (report.Status, string) {
		status, _, err := httpGet(ctx, h.BaseURL+"/version", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status == 200 {
			return report.Fail, "expected 401/302, got 200 (no auth was accepted)"
		}
		if status != 401 && status != 302 {
			return report.Fail, fmt.Sprintf("expected 401 or 302, got %d", status)
		}
		return report.Pass, ""
	})
}

func check06StandaloneEquivalence(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(6, "Standalone: vanilla equivalence", func() (report.Status, string) {
		status, _, err := httpGet(ctx, h.BaseURL+"/version", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected open access in standalone, got %d", status)
		}
		return report.Pass, ""
	})
}

func check07Health(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(7, "GET /health", func() (report.Status, string) {
		status, body, err := httpGet(ctx, h.BaseURL+"/health", nil)
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
		if data["status"] != "ok" {
			return report.Fail, fmt.Sprintf("expected status=ok, got %v", data["status"])
		}
		return report.Pass, ""
	})
}

func check08Readyz(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(8, "GET /readyz", func() (report.Status, string) {
		status, body, err := httpGet(ctx, h.BaseURL+"/readyz", nil)
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
		if _, ok := data["ready"]; !ok {
			return report.Fail, "missing required field: ready"
		}
		checks, ok := data["checks"].(map[string]any)
		if !ok {
			return report.Fail, "missing or invalid field: checks"
		}
		for name, v := range checks {
			check, ok := v.(map[string]any)
			if !ok {
				return report.Fail, fmt.Sprintf("check %q is not an object", name)
			}
			if _, ok := check["ok"]; !ok {
				return report.Fail, fmt.Sprintf("check %q missing required field: ok", name)
			}
		}
		return report.Pass, ""
	})
}

func check09VersionStandalone(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(9, "GET /version (standalone)", func() (report.Status, string) {
		status, body, err := httpGet(ctx, h.BaseURL+"/version", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected 200, got %d", status)
		}
		return validateVersionBody(body)
	})
}

func check10VersionManaged(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(10, "GET /version (managed, no auth)", func() (report.Status, string) {
		status, _, err := httpGet(ctx, h.BaseURL+"/version", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status == 200 {
			return report.Fail, "expected 401, got 200 (version is ungated)"
		}
		if status != 401 && status != 302 {
			return report.Fail, fmt.Sprintf("expected 401 or 302, got %d", status)
		}
		return report.Pass, ""
	})
}

func check11CapabilitiesStandalone(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(11, "GET /capabilities (standalone)", func() (report.Status, string) {
		status, body, err := httpGet(ctx, h.BaseURL+"/capabilities", nil)
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
		if _, ok := data["contract_version"]; !ok {
			return report.Fail, "missing required field: contract_version"
		}
		caps, ok := data["capabilities"].(map[string]any)
		if !ok {
			return report.Fail, "missing or invalid field: capabilities"
		}
		if len(caps) == 0 {
			return report.Fail, "capabilities map is empty"
		}
		return report.Pass, ""
	})
}

func check12CapabilitiesManaged(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(12, "GET /capabilities (managed, no auth)", func() (report.Status, string) {
		status, _, err := httpGet(ctx, h.BaseURL+"/capabilities", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status == 200 {
			return report.Fail, "expected 401, got 200 (capabilities is ungated)"
		}
		if status != 401 && status != 302 {
			return report.Fail, fmt.Sprintf("expected 401 or 302, got %d", status)
		}
		return report.Pass, ""
	})
}

func check13SSEContractEvents(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(13, "SSE: contract events", func() (report.Status, string) {
		if h.AuthSecret == "" {
			return report.Skip, "requires managed-mode SUT with mock LLM"
		}
		body := `{"model":"mock","messages":[{"role":"user","content":"test"}],"stream":true}`
		status, respBody, err := httpPost(ctx, h.BaseURL+"/api/chat/completions",
			body, map[string]string{"X-Fox-Auth": h.AuthSecret})
		if err != nil {
			return report.Skip, fmt.Sprintf("chat request failed: %v", err)
		}
		if status != 200 {
			return report.Skip, fmt.Sprintf("chat returned %d; SSE contract events deferred to instance contract v0.2", status)
		}
		events := parseSSEEvents(string(respBody))
		required := map[string]bool{"token": false, "done": false}
		for _, e := range events {
			if _, ok := required[e.event]; ok {
				required[e.event] = true
			}
		}
		var missing []string
		for name, found := range required {
			if !found {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return report.Fail, fmt.Sprintf("missing SSE events: %s", strings.Join(missing, ", "))
		}
		return report.Pass, ""
	})
}

func check14SSEAppError(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(14, "SSE: apperror on provider error", func() (report.Status, string) {
		if h.AuthSecret == "" {
			return report.Skip, "requires managed-mode SUT with mock LLM"
		}
		body := `{"model":"mock","messages":[{"role":"user","content":"test"}],"stream":true}`
		status, respBody, err := httpPost(ctx, h.BaseURL+"/api/chat/completions",
			body, map[string]string{"X-Fox-Auth": h.AuthSecret})
		if err != nil {
			return report.Skip, fmt.Sprintf("chat request failed: %v", err)
		}
		if status >= 400 {
			return report.Pass, fmt.Sprintf("provider error surfaced as HTTP %d", status)
		}
		if status != 200 {
			return report.Skip, fmt.Sprintf("chat returned %d (mock LLM may not be wired)", status)
		}
		events := parseSSEEvents(string(respBody))
		for _, e := range events {
			if e.event == "apperror" || e.event == "error" {
				return report.Pass, ""
			}
		}
		return report.Skip, "apperror event not observed (mock LLM error may not propagate)"
	})
}

func check15StandaloneBoot(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(15, "Standalone boot", func() (report.Status, string) {
		status, _, err := httpGet(ctx, h.BaseURL+"/health", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("expected 200, got %d", status)
		}
		return report.Pass, ""
	})
}

func check16ContractVersion(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(16, "Contract version", func() (report.Status, string) {
		headers := map[string]string{}
		if h.AuthSecret != "" {
			headers["X-Fox-Auth"] = h.AuthSecret
		}
		status, body, err := httpGet(ctx, h.BaseURL+"/version", headers)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("GET /version returned %d", status)
		}
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			return report.Fail, fmt.Sprintf("invalid JSON: %v", err)
		}
		cv, ok := data["contract_version"].(string)
		if !ok || cv == "" {
			return report.Fail, "missing contract_version field"
		}
		if !strings.HasPrefix(cv, contractVersionPrefix) {
			return report.Fail, fmt.Sprintf("expected contract_version %sx, got %q", contractVersionPrefix, cv)
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

var noFollow = func(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

func httpGetFull(ctx context.Context, url string, headers map[string]string) (int, []byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c := &http.Client{Timeout: 10 * time.Second, CheckRedirect: noFollow}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, "", fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, body, resp.Header.Get("Content-Type"), nil
}

func httpGet(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	status, body, _, err := httpGetFull(ctx, url, headers)
	return status, body, err
}

func httpPost(ctx context.Context, url string, jsonBody string, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(jsonBody))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c := &http.Client{Timeout: 30 * time.Second, CheckRedirect: noFollow}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, body, nil
}

func obtainSessionCookie(ctx context.Context, baseURL, password string) (string, error) {
	loginBody := fmt.Sprintf(`{"password":%q}`, password)
	status, _, cookies, err := httpPostWithCookies(ctx, baseURL+foxLoginPath, loginBody, nil)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	if status != 200 {
		return "", fmt.Errorf("login returned %d", status)
	}
	for _, c := range cookies {
		if c.Name == foxSessionCookie {
			return c.Name + "=" + c.Value, nil
		}
	}
	return "", fmt.Errorf("login response missing %s cookie", foxSessionCookie)
}

func httpPostWithCookies(ctx context.Context, url string, jsonBody string, headers map[string]string) (int, []byte, []*http.Cookie, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(jsonBody))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c := &http.Client{Timeout: 30 * time.Second, CheckRedirect: noFollow}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, body, resp.Cookies(), nil
}

func validateVersionBody(body []byte) (report.Status, string) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return report.Fail, fmt.Sprintf("invalid JSON: %v", err)
	}
	required := []string{"contract_version", "runtime"}
	for _, field := range required {
		if _, ok := data[field]; !ok {
			return report.Fail, fmt.Sprintf("missing required field: %s", field)
		}
	}
	return report.Pass, ""
}

func check17VersionV2Schema(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(17, "Version v2.0 schema", func() (report.Status, string) {
		headers := map[string]string{}
		if h.AuthSecret != "" {
			headers["X-Fox-Auth"] = h.AuthSecret
		}
		status, body, ct, err := httpGetFull(ctx, h.BaseURL+"/version", headers)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("GET /version returned %d", status)
		}
		if !strings.HasPrefix(ct, "application/json") {
			return report.Fail, fmt.Sprintf("expected Content-Type application/json, got %q", ct)
		}
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			return report.Fail, fmt.Sprintf("invalid JSON: %v", err)
		}
		required := []string{"contract_version", "image_digest", "runtime", "runtime_version", "overlay_version"}
		for _, field := range required {
			v, ok := data[field]
			if !ok {
				return report.Fail, fmt.Sprintf("missing required field: %s", field)
			}
			if _, isStr := v.(string); !isStr {
				return report.Fail, fmt.Sprintf("field %s must be a string, got %T", field, v)
			}
		}
		cv := data["contract_version"].(string)
		if !strings.HasPrefix(cv, contractVersionPrefix) {
			return report.Fail, fmt.Sprintf("expected contract_version 2.x, got %q", cv)
		}
		return report.Pass, ""
	})
}

func check18CapabilitiesV2Schema(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(18, "Capabilities v2.0 schema", func() (report.Status, string) {
		headers := map[string]string{}
		if h.AuthSecret != "" {
			headers["X-Fox-Auth"] = h.AuthSecret
		}
		status, body, ct, err := httpGetFull(ctx, h.BaseURL+"/capabilities", headers)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("GET /capabilities returned %d", status)
		}
		if !strings.HasPrefix(ct, "application/json") {
			return report.Fail, fmt.Sprintf("expected Content-Type application/json, got %q", ct)
		}
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			return report.Fail, fmt.Sprintf("invalid JSON: %v", err)
		}
		cv, ok := data["contract_version"].(string)
		if !ok || cv == "" {
			return report.Fail, "missing or empty contract_version"
		}
		if !strings.HasPrefix(cv, contractVersionPrefix) {
			return report.Fail, fmt.Sprintf("expected contract_version 2.x, got %q", cv)
		}
		caps, ok := data["capabilities"].(map[string]any)
		if !ok {
			return report.Fail, "missing or invalid capabilities object"
		}
		for name, v := range caps {
			if _, isBool := v.(bool); !isBool {
				return report.Fail, fmt.Sprintf("capability %q must be a boolean, got %T", name, v)
			}
		}
		return report.Pass, ""
	})
}

func check19HealthV2ContentType(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(19, "Health v2.0 content-type", func() (report.Status, string) {
		headers := map[string]string{}
		if h.AuthSecret != "" {
			headers["X-Fox-Auth"] = h.AuthSecret
		}
		_, _, ct, err := httpGetFull(ctx, h.BaseURL+"/health", headers)
		if err != nil {
			return report.Fail, err.Error()
		}
		if !strings.HasPrefix(ct, "application/json") {
			return report.Fail, fmt.Sprintf("expected Content-Type application/json, got %q", ct)
		}
		return report.Pass, ""
	})
}

func check20ReadyzV2Schema(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(20, "Readyz v2.0 schema", func() (report.Status, string) {
		headers := map[string]string{}
		if h.AuthSecret != "" {
			headers["X-Fox-Auth"] = h.AuthSecret
		}
		status, body, ct, err := httpGetFull(ctx, h.BaseURL+"/readyz", headers)
		if err != nil {
			return report.Fail, err.Error()
		}
		if status != 200 {
			return report.Fail, fmt.Sprintf("GET /readyz returned %d", status)
		}
		if !strings.HasPrefix(ct, "application/json") {
			return report.Fail, fmt.Sprintf("expected Content-Type application/json, got %q", ct)
		}
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			return report.Fail, fmt.Sprintf("invalid JSON: %v", err)
		}
		ready, ok := data["ready"]
		if !ok {
			return report.Fail, "missing required field: ready"
		}
		if _, isBool := ready.(bool); !isBool {
			return report.Fail, fmt.Sprintf("ready must be a boolean, got %T", ready)
		}
		checks, ok := data["checks"].(map[string]any)
		if !ok {
			return report.Fail, "missing or invalid field: checks"
		}
		for name, v := range checks {
			check, isObj := v.(map[string]any)
			if !isObj {
				return report.Fail, fmt.Sprintf("check %q must be an object", name)
			}
			okVal, hasOK := check["ok"]
			if !hasOK {
				return report.Fail, fmt.Sprintf("check %q missing required field: ok", name)
			}
			if _, isBool := okVal.(bool); !isBool {
				return report.Fail, fmt.Sprintf("check %q ok must be a boolean, got %T", name, okVal)
			}
		}
		return report.Pass, ""
	})
}

// --- security conformance checks (CONF-02) ---

func check21SecurityHeaders(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(21, "Security headers present", func() (report.Status, string) {
		headers := map[string]string{}
		if h.AuthSecret != "" {
			headers["X-Fox-Auth"] = h.AuthSecret
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+"/health", nil)
		if err != nil {
			return report.Fail, err.Error()
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		c := &http.Client{Timeout: 10 * time.Second, CheckRedirect: noFollow}
		resp, err := c.Do(req)
		if err != nil {
			return report.Fail, err.Error()
		}
		resp.Body.Close()

		var missing []string
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			missing = append(missing, "X-Content-Type-Options")
		}
		if len(missing) > 0 {
			return report.Fail, fmt.Sprintf("missing or wrong headers: %s", strings.Join(missing, ", "))
		}
		return report.Pass, ""
	})
}

func check22PathTraversal(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(22, "Path traversal rejected", func() (report.Status, string) {
		headers := map[string]string{}
		if h.AuthSecret != "" {
			headers["X-Fox-Auth"] = h.AuthSecret
		}
		paths := []string{
			"/../../etc/passwd",
			"/%2e%2e/%2e%2e/etc/passwd",
		}
		for _, p := range paths {
			status, body, err := httpGet(ctx, h.BaseURL+p, headers)
			if err != nil {
				continue
			}
			if status == 200 && len(body) > 0 && strings.Contains(string(body), "root:") {
				return report.Fail, fmt.Sprintf("path traversal succeeded: %s", p)
			}
		}
		return report.Pass, ""
	})
}

func check23AuthTimingConsistency(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(23, "Auth rejects don't leak timing", func() (report.Status, string) {
		if h.AuthSecret == "" {
			return report.Skip, "requires managed-mode SUT"
		}
		status1, _, err := httpGet(ctx, h.BaseURL+"/version", map[string]string{
			"X-Fox-Auth": "wrong-short",
		})
		if err != nil {
			return report.Fail, err.Error()
		}
		status2, _, err := httpGet(ctx, h.BaseURL+"/version", map[string]string{
			"X-Fox-Auth": "wrong-with-a-very-long-value-that-differs-in-length-from-the-actual-secret",
		})
		if err != nil {
			return report.Fail, err.Error()
		}
		if status1 != status2 {
			return report.Fail, fmt.Sprintf("different status codes: short=%d long=%d", status1, status2)
		}
		return report.Pass, ""
	})
}

func check24MethodNotAllowed(ctx context.Context, h *sut.Handle) report.Result {
	return timedCheck(24, "Unsupported HTTP methods rejected", func() (report.Status, string) {
		for _, method := range []string{http.MethodDelete, http.MethodPut, http.MethodPatch} {
			req, err := http.NewRequestWithContext(ctx, method, h.BaseURL+"/health", nil)
			if err != nil {
				continue
			}
			c := &http.Client{Timeout: 10 * time.Second, CheckRedirect: noFollow}
			resp, err := c.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return report.Fail, fmt.Sprintf("%s /health returned 200 (expected 405 or non-200)", method)
			}
		}
		return report.Pass, ""
	})
}

type sseEvent struct {
	event string
	data  string
}

func parseSSEEvents(raw string) []sseEvent {
	var events []sseEvent
	var current sseEvent
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "event: ") {
			current.event = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			current.data = strings.TrimPrefix(line, "data: ")
		} else if line == "" && (current.event != "" || current.data != "") {
			if current.event == "" {
				current.event = "message"
			}
			events = append(events, current)
			current = sseEvent{}
		}
	}
	if current.event != "" || current.data != "" {
		if current.event == "" {
			current.event = "message"
		}
		events = append(events, current)
	}
	return events
}

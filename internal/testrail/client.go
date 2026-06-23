package testrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/achmadsalmanr/mcp-server/internal/scm"
)

type Client struct {
	baseURL string
	email   string
	apiKey  string
	http    *http.Client
}

type ListCasesInput struct {
	ProjectID int `json:"project_id"`
	SuiteID   int `json:"suite_id,omitempty"`
	SectionID int `json:"section_id,omitempty"`
	Limit     int `json:"limit,omitempty"`
	Offset    int `json:"offset,omitempty"`
}

type ListTestsInput struct {
	RunID  int `json:"run_id"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type AddResultInput struct {
	TestID     int    `json:"test_id"`
	StatusID   int    `json:"status_id"`
	Comment    string `json:"comment,omitempty"`
	Version    string `json:"version,omitempty"`
	Elapsed    string `json:"elapsed,omitempty"`
	Defects    string `json:"defects,omitempty"`
	AssignedTo int    `json:"assignedto_id,omitempty"`
}

type AddResultForCaseInput struct {
	RunID      int    `json:"run_id"`
	CaseID     int    `json:"case_id"`
	StatusID   int    `json:"status_id"`
	Comment    string `json:"comment,omitempty"`
	Version    string `json:"version,omitempty"`
	Elapsed    string `json:"elapsed,omitempty"`
	Defects    string `json:"defects,omitempty"`
	AssignedTo int    `json:"assignedto_id,omitempty"`
}

func NewClient(baseURL, email, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		email:   email,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.email != "" && c.apiKey != ""
}

func (c *Client) GetCase(ctx context.Context, caseID int) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "get_case/"+strconv.Itoa(caseID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListCases(ctx context.Context, input ListCasesInput) ([]map[string]any, error) {
	path := "get_cases/" + strconv.Itoa(input.ProjectID)
	q := make(url.Values)
	if input.SuiteID != 0 {
		q.Set("suite_id", strconv.Itoa(input.SuiteID))
	}
	if input.SectionID != 0 {
		q.Set("section_id", strconv.Itoa(input.SectionID))
	}
	if input.Limit != 0 {
		q.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Offset != 0 {
		q.Set("offset", strconv.Itoa(input.Offset))
	}
	path = appendTestRailQuery(path, q)
	raw, err := c.raw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeList(raw, "cases")
}

func (c *Client) GetRun(ctx context.Context, runID int) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "get_run/"+strconv.Itoa(runID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListTests(ctx context.Context, input ListTestsInput) ([]map[string]any, error) {
	path := "get_tests/" + strconv.Itoa(input.RunID)
	q := make(url.Values)
	if input.Limit != 0 {
		q.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Offset != 0 {
		q.Set("offset", strconv.Itoa(input.Offset))
	}
	path = appendTestRailQuery(path, q)
	raw, err := c.raw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeList(raw, "tests")
}

func (c *Client) AddResult(ctx context.Context, input AddResultInput) (map[string]any, error) {
	payload := resultPayload(input.StatusID, input.Comment, input.Version, input.Elapsed, input.Defects, input.AssignedTo)
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "add_result/"+strconv.Itoa(input.TestID), payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) AddResultForCase(ctx context.Context, input AddResultForCaseInput) (map[string]any, error) {
	payload := resultPayload(input.StatusID, input.Comment, input.Version, input.Elapsed, input.Defects, input.AssignedTo)
	path := fmt.Sprintf("add_result_for_case/%d/%d", input.RunID, input.CaseID)
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, path, payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func resultPayload(statusID int, comment, version, elapsed, defects string, assignedTo int) map[string]any {
	payload := map[string]any{"status_id": statusID}
	if comment != "" {
		payload["comment"] = comment
	}
	if version != "" {
		payload["version"] = version
	}
	if elapsed != "" {
		payload["elapsed"] = elapsed
	}
	if defects != "" {
		payload["defects"] = defects
	}
	if assignedTo != 0 {
		payload["assignedto_id"] = assignedTo
	}
	return payload
}

func appendTestRailQuery(path string, q url.Values) string {
	encoded := q.Encode()
	if encoded == "" {
		return path
	}
	return path + "&" + encoded
}

func decodeList(raw []byte, field string) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if value, ok := obj[field]; ok {
		if err := json.Unmarshal(value, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}
	return nil, fmt.Errorf("response did not contain array or %q field", field)
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	raw, err := c.raw(ctx, method, path, body)
	if err != nil {
		return err
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decoding TestRail response from %s: %w", path, err)
		}
	}
	return nil
}

func (c *Client) raw(ctx context.Context, method, path string, body any) ([]byte, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("testrail is not configured; set TESTRAIL_BASE_URL, TESTRAIL_EMAIL, and TESTRAIL_API_KEY")
	}
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	reqURL := c.baseURL + "/index.php?/api/v2/" + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("building TestRail request: %w", err)
	}
	req.SetBasicAuth(c.email, c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, scm.MaxBodyRead))
		return nil, &scm.APIError{Provider: "testrail", StatusCode: resp.StatusCode, Path: path, Body: string(b)}
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading TestRail response: %w", err)
	}
	return b, nil
}

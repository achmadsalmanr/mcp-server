package figma

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

	"github.com/achmadsalmanr/mcp-server/internal/scm"
)

const defaultAPIBase = "https://api.figma.com"

type Client struct {
	token   string
	apiBase string
	http    *http.Client
}

type FileRef struct {
	Key     string
	NodeIDs []string
}

type GetFileInput struct {
	FileKey    string   `json:"file_key"`
	Version    string   `json:"version,omitempty"`
	IDs        []string `json:"ids,omitempty"`
	Depth      int      `json:"depth,omitempty"`
	Geometry   string   `json:"geometry,omitempty"`
	PluginData []string `json:"plugin_data,omitempty"`
	BranchData bool     `json:"branch_data,omitempty"`
}

type GetFileNodesInput struct {
	FileKey    string   `json:"file_key"`
	IDs        []string `json:"ids,omitempty"`
	Version    string   `json:"version,omitempty"`
	Depth      int      `json:"depth,omitempty"`
	Geometry   string   `json:"geometry,omitempty"`
	PluginData []string `json:"plugin_data,omitempty"`
}

type GetImagesInput struct {
	FileKey           string   `json:"file_key"`
	IDs               []string `json:"ids,omitempty"`
	Scale             float64  `json:"scale,omitempty"`
	Format            string   `json:"format,omitempty"`
	SVGOutlineText    *bool    `json:"svg_outline_text,omitempty"`
	SVGIncludeID      *bool    `json:"svg_include_id,omitempty"`
	SVGIncludeNodeID  *bool    `json:"svg_include_node_id,omitempty"`
	SVGSimplifyStroke *bool    `json:"svg_simplify_stroke,omitempty"`
	ContentsOnly      *bool    `json:"contents_only,omitempty"`
	UseAbsoluteBounds *bool    `json:"use_absolute_bounds,omitempty"`
	Version           string   `json:"version,omitempty"`
}

type GetCommentsInput struct {
	FileKey string `json:"file_key"`
	AsMD    bool   `json:"as_md,omitempty"`
}

type PostCommentInput struct {
	FileKey    string         `json:"file_key"`
	Message    string         `json:"message"`
	CommentID  string         `json:"comment_id,omitempty"`
	ClientMeta map[string]any `json:"client_meta,omitempty"`
}

func NewClient(token, apiBase string) *Client {
	if strings.TrimSpace(apiBase) == "" {
		apiBase = defaultAPIBase
	}
	return &Client{
		token:   strings.TrimSpace(token),
		apiBase: strings.TrimRight(strings.TrimSpace(apiBase), "/"),
		http:    scm.NewHTTPClient(),
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.token != ""
}

func (c *Client) GetFile(ctx context.Context, input GetFileInput) (map[string]any, error) {
	ref, err := ParseFileRef(input.FileKey)
	if err != nil {
		return nil, err
	}
	q := commonFileQuery(input.Version, input.Depth, input.Geometry, input.PluginData)
	if ids := normalizedIDs(append(input.IDs, ref.NodeIDs...)); len(ids) > 0 {
		q.Set("ids", strings.Join(ids, ","))
	}
	if input.BranchData {
		q.Set("branch_data", "true")
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/files/"+url.PathEscape(ref.Key), q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetFileNodes(ctx context.Context, input GetFileNodesInput) (map[string]any, error) {
	ref, err := ParseFileRef(input.FileKey)
	if err != nil {
		return nil, err
	}
	ids := normalizedIDs(append(input.IDs, ref.NodeIDs...))
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids is required; pass node IDs or a Figma URL with node-id")
	}
	q := commonFileQuery(input.Version, input.Depth, input.Geometry, input.PluginData)
	q.Set("ids", strings.Join(ids, ","))
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/files/"+url.PathEscape(ref.Key)+"/nodes", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetImages(ctx context.Context, input GetImagesInput) (map[string]any, error) {
	ref, err := ParseFileRef(input.FileKey)
	if err != nil {
		return nil, err
	}
	ids := normalizedIDs(append(input.IDs, ref.NodeIDs...))
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids is required; pass node IDs or a Figma URL with node-id")
	}
	q := make(url.Values)
	q.Set("ids", strings.Join(ids, ","))
	if input.Scale != 0 {
		q.Set("scale", strconv.FormatFloat(input.Scale, 'f', -1, 64))
	}
	if input.Format != "" {
		q.Set("format", input.Format)
	}
	addBoolQuery(q, "svg_outline_text", input.SVGOutlineText)
	addBoolQuery(q, "svg_include_id", input.SVGIncludeID)
	addBoolQuery(q, "svg_include_node_id", input.SVGIncludeNodeID)
	addBoolQuery(q, "svg_simplify_stroke", input.SVGSimplifyStroke)
	addBoolQuery(q, "contents_only", input.ContentsOnly)
	addBoolQuery(q, "use_absolute_bounds", input.UseAbsoluteBounds)
	if input.Version != "" {
		q.Set("version", input.Version)
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/images/"+url.PathEscape(ref.Key), q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetImageFills(ctx context.Context, fileKey string) (map[string]any, error) {
	ref, err := ParseFileRef(fileKey)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/files/"+url.PathEscape(ref.Key)+"/images", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetComments(ctx context.Context, input GetCommentsInput) (map[string]any, error) {
	ref, err := ParseFileRef(input.FileKey)
	if err != nil {
		return nil, err
	}
	q := make(url.Values)
	if input.AsMD {
		q.Set("as_md", "true")
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/files/"+url.PathEscape(ref.Key)+"/comments", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) PostComment(ctx context.Context, input PostCommentInput) (map[string]any, error) {
	ref, err := ParseFileRef(input.FileKey)
	if err != nil {
		return nil, err
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	payload := map[string]any{"message": message}
	if input.CommentID != "" {
		payload["comment_id"] = input.CommentID
	}
	if len(input.ClientMeta) > 0 {
		payload["client_meta"] = input.ClientMeta
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/v1/files/"+url.PathEscape(ref.Key)+"/comments", nil, payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func ParseFileRef(raw string) (FileRef, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return FileRef{}, fmt.Errorf("file_key is required")
	}
	if !strings.Contains(value, "://") {
		if !scm.ValidPathSegment(value) {
			return FileRef{}, fmt.Errorf("invalid Figma file key %q", value)
		}
		return FileRef{Key: value}, nil
	}
	u, err := url.Parse(value)
	if err != nil {
		return FileRef{}, fmt.Errorf("invalid Figma URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if !isFigmaHost(host) {
		return FileRef{}, fmt.Errorf("URL host must be figma.com or figma-gov.com, got %q", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[1] == "" {
		return FileRef{}, fmt.Errorf("URL does not contain a Figma file key: %q", raw)
	}
	if !scm.ValidPathSegment(parts[1]) {
		return FileRef{}, fmt.Errorf("invalid Figma file key in URL: %q", raw)
	}
	ref := FileRef{Key: parts[1]}
	if nodeID := u.Query().Get("node-id"); nodeID != "" {
		ref.NodeIDs = []string{normalizeNodeID(nodeID)}
	}
	return ref, nil
}

func commonFileQuery(version string, depth int, geometry string, pluginData []string) url.Values {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if depth != 0 {
		q.Set("depth", strconv.Itoa(depth))
	}
	if geometry != "" {
		q.Set("geometry", geometry)
	}
	if values := normalizedStrings(pluginData); len(values) > 0 {
		q.Set("plugin_data", strings.Join(values, ","))
	}
	return q
}

func addBoolQuery(q url.Values, key string, value *bool) {
	if value != nil {
		q.Set(key, strconv.FormatBool(*value))
	}
}

func normalizedIDs(values []string) []string {
	values = normalizedStrings(values)
	for i, value := range values {
		values[i] = normalizeNodeID(value)
	}
	return values
}

func normalizedStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeNodeID(value string) string {
	if !strings.Contains(value, ":") && strings.Contains(value, "-") {
		return strings.ReplaceAll(value, "-", ":")
	}
	return value
}

func isFigmaHost(host string) bool {
	return host == "figma.com" || strings.HasSuffix(host, ".figma.com") ||
		host == "figma-gov.com" || strings.HasSuffix(host, ".figma-gov.com")
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	raw, err := c.raw(ctx, method, path, q, body)
	if err != nil {
		return err
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decoding Figma response from %s: %w", path, err)
		}
	}
	return nil
}

func (c *Client) raw(ctx context.Context, method, path string, q url.Values, body any) ([]byte, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("figma is not configured; set FIGMA_ACCESS_TOKEN")
	}
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	reqURL, err := url.Parse(c.apiBase + path)
	if err != nil {
		return nil, fmt.Errorf("building Figma request URL: %w", err)
	}
	if len(q) > 0 {
		reqURL.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("building Figma request: %w", err)
	}
	req.Header.Set("X-Figma-Token", c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, scm.MaxBodyRead))
		return nil, &scm.APIError{Provider: "figma", StatusCode: resp.StatusCode, Path: path, Body: string(b)}
	}
	b, err := scm.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Figma response: %w", err)
	}
	return b, nil
}

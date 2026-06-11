package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

func init() {
	registerBuiltins(
		map[string]BuiltinTool{
			"url_parse":        &URLParseTool{},
			"url_build":        &URLBuildTool{},
			"url_query_add":    &URLQueryAddTool{},
			"url_query_remove": &URLQueryRemoveTool{},
			"url_query_get":    &URLQueryGetTool{},
			"url_normalize":    &URLNormalizeTool{},
			"ip_validate":      &IPValidateTool{},
			"ip_in_cidr":       &IPInCIDRTool{},
			"cidr_parse":       &CIDRParseTool{},
			"domain_extract":   &DomainExtractTool{},
			"email_validate":   &EmailValidateTool{},
			"user_agent_parse": &UserAgentParseTool{},
		},
		map[string]bool{
			"url_parse":        true,
			"url_build":        true,
			"url_query_add":    true,
			"url_query_remove": true,
			"url_query_get":    true,
			"url_normalize":    true,
			"ip_validate":      true,
			"ip_in_cidr":       true,
			"cidr_parse":       true,
			"domain_extract":   true,
			"email_validate":   true,
			"user_agent_parse": true,
		},
	)
}

type URLParseTool struct{}

func (t *URLParseTool) Name() string        { return "url_parse" }
func (t *URLParseTool) Description() string { return "Parse URL into components" }
func (t *URLParseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "URL to parse", "default": "https://example.com"},
		},
	}
}
func (t *URLParseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	urlStr, _ := args["url"].(string)
	if urlStr == "" {
		urlStr = "https://example.com"
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid URL: %v", err), IsError: true}, nil
	}
	result := map[string]string{
		"scheme":   u.Scheme,
		"host":     u.Host,
		"path":     u.Path,
		"query":    u.RawQuery,
		"fragment": u.Fragment,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &ToolResult{Content: string(data)}, nil
}

type URLBuildTool struct{}

func (t *URLBuildTool) Name() string        { return "url_build" }
func (t *URLBuildTool) Description() string { return "Build URL from components" }
func (t *URLBuildTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"scheme":   map[string]any{"type": "string", "description": "URL scheme", "default": "https"},
			"host":     map[string]any{"type": "string", "description": "Host", "default": "example.com"},
			"path":     map[string]any{"type": "string", "description": "Path", "default": "/"},
			"query":    map[string]any{"type": "object", "description": "Query parameters"},
			"fragment": map[string]any{"type": "string", "description": "Fragment"},
		},
	}
}
func (t *URLBuildTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	scheme, _ := args["scheme"].(string)
	host, _ := args["host"].(string)
	path, _ := args["path"].(string)
	fragment, _ := args["fragment"].(string)
	if scheme == "" {
		scheme = "https"
	}
	if host == "" {
		host = "example.com"
	}
	if path == "" {
		path = "/"
	}
	u := url.URL{Scheme: scheme, Host: host, Path: path, Fragment: fragment}
	if queryMap, ok := args["query"].(map[string]any); ok {
		q := url.Values{}
		for k, v := range queryMap {
			q.Set(k, fmt.Sprint(v))
		}
		u.RawQuery = q.Encode()
	}
	return &ToolResult{Content: u.String()}, nil
}

type URLQueryAddTool struct{}

func (t *URLQueryAddTool) Name() string        { return "url_query_add" }
func (t *URLQueryAddTool) Description() string { return "Add query parameter to URL" }
func (t *URLQueryAddTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":   map[string]any{"type": "string", "description": "URL", "default": "https://example.com"},
			"key":   map[string]any{"type": "string", "description": "Parameter key", "default": "q"},
			"value": map[string]any{"type": "string", "description": "Parameter value", "default": "test"},
		},
	}
}
func (t *URLQueryAddTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	urlStr, _ := args["url"].(string)
	key, _ := args["key"].(string)
	value, _ := args["value"].(string)
	if urlStr == "" {
		urlStr = "https://example.com"
	}
	if key == "" {
		key = "q"
	}
	if value == "" {
		value = "test"
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid URL: %v", err), IsError: true}, nil
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return &ToolResult{Content: u.String()}, nil
}

type URLQueryRemoveTool struct{}

func (t *URLQueryRemoveTool) Name() string        { return "url_query_remove" }
func (t *URLQueryRemoveTool) Description() string { return "Remove query parameter from URL" }
func (t *URLQueryRemoveTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "URL", "default": "https://example.com?q=test"},
			"key": map[string]any{"type": "string", "description": "Parameter key", "default": "q"},
		},
	}
}
func (t *URLQueryRemoveTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	urlStr, _ := args["url"].(string)
	key, _ := args["key"].(string)
	if urlStr == "" {
		urlStr = "https://example.com?q=test"
	}
	if key == "" {
		key = "q"
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid URL: %v", err), IsError: true}, nil
	}
	q := u.Query()
	q.Del(key)
	u.RawQuery = q.Encode()
	return &ToolResult{Content: u.String()}, nil
}

type URLQueryGetTool struct{}

func (t *URLQueryGetTool) Name() string        { return "url_query_get" }
func (t *URLQueryGetTool) Description() string { return "Get query parameter value" }
func (t *URLQueryGetTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "URL", "default": "https://example.com?q=test"},
			"key": map[string]any{"type": "string", "description": "Parameter key", "default": "q"},
		},
	}
}
func (t *URLQueryGetTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	urlStr, _ := args["url"].(string)
	key, _ := args["key"].(string)
	if urlStr == "" {
		urlStr = "https://example.com?q=test"
	}
	if key == "" {
		key = "q"
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid URL: %v", err), IsError: true}, nil
	}
	value := u.Query().Get(key)
	return &ToolResult{Content: value}, nil
}

type URLNormalizeTool struct{}

func (t *URLNormalizeTool) Name() string        { return "url_normalize" }
func (t *URLNormalizeTool) Description() string { return "Normalize URL (canonical form)" }
func (t *URLNormalizeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "URL to normalize", "default": "HTTP://Example.COM:80/path?b=2&a=1"},
		},
	}
}
func (t *URLNormalizeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	urlStr, _ := args["url"].(string)
	if urlStr == "" {
		urlStr = "HTTP://Example.COM:80/path?b=2&a=1"
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid URL: %v", err), IsError: true}, nil
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) || (u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = u.Host[:strings.LastIndex(u.Host, ":")]
	}
	q := u.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sorted := url.Values{}
	for _, k := range keys {
		sorted[k] = q[k]
	}
	u.RawQuery = sorted.Encode()
	return &ToolResult{Content: u.String()}, nil
}

type IPValidateTool struct{}

func (t *IPValidateTool) Name() string        { return "ip_validate" }
func (t *IPValidateTool) Description() string { return "Validate IP address format" }
func (t *IPValidateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ip": map[string]any{"type": "string", "description": "IP address", "default": "192.168.1.1"},
		},
	}
}
func (t *IPValidateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	ipStr, _ := args["ip"].(string)
	if ipStr == "" {
		ipStr = "192.168.1.1"
	}
	ip := net.ParseIP(ipStr)
	result := map[string]any{"valid": ip != nil}
	if ip != nil {
		if ip.To4() != nil {
			result["version"] = 4
		} else {
			result["version"] = 6
		}
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &ToolResult{Content: string(data)}, nil
}

type IPInCIDRTool struct{}

func (t *IPInCIDRTool) Name() string        { return "ip_in_cidr" }
func (t *IPInCIDRTool) Description() string { return "Check if IP is in CIDR range" }
func (t *IPInCIDRTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ip":   map[string]any{"type": "string", "description": "IP address", "default": "192.168.1.1"},
			"cidr": map[string]any{"type": "string", "description": "CIDR notation", "default": "192.168.1.0/24"},
		},
	}
}
func (t *IPInCIDRTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	ipStr, _ := args["ip"].(string)
	cidrStr, _ := args["cidr"].(string)
	if ipStr == "" {
		ipStr = "192.168.1.1"
	}
	if cidrStr == "" {
		cidrStr = "192.168.1.0/24"
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return &ToolResult{Content: "invalid IP address", IsError: true}, nil
	}
	_, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid CIDR: %v", err), IsError: true}, nil
	}
	if ipNet.Contains(ip) {
		return &ToolResult{Content: "true"}, nil
	}
	return &ToolResult{Content: "false"}, nil
}

type CIDRParseTool struct{}

func (t *CIDRParseTool) Name() string        { return "cidr_parse" }
func (t *CIDRParseTool) Description() string { return "Parse CIDR notation" }
func (t *CIDRParseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cidr": map[string]any{"type": "string", "description": "CIDR notation", "default": "192.168.1.0/24"},
		},
	}
}
func (t *CIDRParseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	cidrStr, _ := args["cidr"].(string)
	if cidrStr == "" {
		cidrStr = "192.168.1.0/24"
	}
	ip, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid CIDR: %v", err), IsError: true}, nil
	}
	ones, bits := ipNet.Mask.Size()
	total := uint64(1) << uint(bits-ones)
	first := ip.Mask(ipNet.Mask)
	last := make(net.IP, len(first))
	copy(last, first)
	for i := range last {
		last[i] |= ^ipNet.Mask[i]
	}
	result := map[string]any{
		"network":   ipNet.IP.String(),
		"broadcast": last.String(),
		"first":     first.String(),
		"last":      last.String(),
		"total":     total,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &ToolResult{Content: string(data)}, nil
}

type DomainExtractTool struct{}

func (t *DomainExtractTool) Name() string        { return "domain_extract" }
func (t *DomainExtractTool) Description() string { return "Extract domain parts from hostname" }
func (t *DomainExtractTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hostname": map[string]any{"type": "string", "description": "Hostname", "default": "www.example.com"},
		},
	}
}
func (t *DomainExtractTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	hostname, _ := args["hostname"].(string)
	if hostname == "" {
		hostname = "www.example.com"
	}
	parts := strings.Split(strings.ToLower(hostname), ".")
	result := map[string]string{"subdomain": "", "domain": "", "tld": ""}
	tlds := map[string]bool{"com": true, "org": true, "net": true, "edu": true, "gov": true, "co": true, "io": true}
	if len(parts) >= 2 {
		if tlds[parts[len(parts)-1]] {
			result["tld"] = parts[len(parts)-1]
			result["domain"] = parts[len(parts)-2]
			if len(parts) > 2 {
				result["subdomain"] = strings.Join(parts[:len(parts)-2], ".")
			}
		}
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &ToolResult{Content: string(data)}, nil
}

type EmailValidateTool struct{}

func (t *EmailValidateTool) Name() string        { return "email_validate" }
func (t *EmailValidateTool) Description() string { return "Validate email address format" }
func (t *EmailValidateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"email": map[string]any{"type": "string", "description": "Email address", "default": "user@example.com"},
		},
	}
}
func (t *EmailValidateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	email, _ := args["email"].(string)
	if email == "" {
		email = "user@example.com"
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	valid := re.MatchString(email)
	result := map[string]any{"valid": valid, "user": "", "domain": ""}
	if valid && strings.Contains(email, "@") {
		parts := strings.SplitN(email, "@", 2)
		result["user"] = parts[0]
		result["domain"] = parts[1]
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &ToolResult{Content: string(data)}, nil
}

type UserAgentParseTool struct{}

func (t *UserAgentParseTool) Name() string        { return "user_agent_parse" }
func (t *UserAgentParseTool) Description() string { return "Parse User-Agent string" }
func (t *UserAgentParseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"user_agent": map[string]any{"type": "string", "description": "User-Agent string", "default": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"},
		},
	}
}
func (t *UserAgentParseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	ua, _ := args["user_agent"].(string)
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	result := map[string]string{"browser": "Unknown", "version": "", "os": "Unknown", "device": "Desktop"}
	if strings.Contains(ua, "Chrome/") {
		result["browser"] = "Chrome"
		if idx := strings.Index(ua, "Chrome/"); idx != -1 {
			ver := ua[idx+7:]
			if end := strings.IndexAny(ver, " ."); end != -1 {
				result["version"] = ver[:end]
			}
		}
	} else if strings.Contains(ua, "Firefox/") {
		result["browser"] = "Firefox"
	} else if strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome") {
		result["browser"] = "Safari"
	}
	if strings.Contains(ua, "Windows") {
		result["os"] = "Windows"
	} else if strings.Contains(ua, "Mac OS") {
		result["os"] = "macOS"
	} else if strings.Contains(ua, "Linux") {
		result["os"] = "Linux"
	} else if strings.Contains(ua, "Android") {
		result["os"] = "Android"
		result["device"] = "Mobile"
	} else if strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") {
		result["os"] = "iOS"
		result["device"] = "Mobile"
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &ToolResult{Content: string(data)}, nil
}

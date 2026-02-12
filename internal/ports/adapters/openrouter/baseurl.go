package openrouter

import (
	"fmt"
	"net/url"
	"strings"
)

const defaultBaseURL = "https://openrouter.ai"

var defaultAllowedHosts = map[string]struct{}{
	"openrouter.ai":     {},
	"api.openrouter.ai": {},
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

func ValidateBaseURL(baseURL string, allowedHosts []string) error {
	baseURL = normalizeBaseURL(baseURL)

	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid OPENROUTER_BASE_URL: %w", err)
	}
	if !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("invalid OPENROUTER_BASE_URL %q: absolute URL with host is required", baseURL)
	}
	if u.User != nil {
		return fmt.Errorf("invalid OPENROUTER_BASE_URL %q: userinfo is not allowed", baseURL)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("invalid OPENROUTER_BASE_URL %q: query and fragment are not allowed", baseURL)
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("invalid OPENROUTER_BASE_URL %q: host is required", baseURL)
	}

	switch scheme {
	case "https":
	default:
		return fmt.Errorf("invalid OPENROUTER_BASE_URL %q: https is required", baseURL)
	}

	allowed := normalizeAllowedHosts(allowedHosts)
	if _, ok := allowed[host]; !ok {
		return fmt.Errorf("invalid OPENROUTER_BASE_URL %q: host %q is not in OPENROUTER_ALLOWED_HOSTS", baseURL, host)
	}
	return nil
}

func normalizeAllowedHosts(allowedHosts []string) map[string]struct{} {
	if len(allowedHosts) == 0 {
		return defaultAllowedHosts
	}

	out := make(map[string]struct{}, len(allowedHosts))
	for _, h := range allowedHosts {
		v := strings.ToLower(strings.TrimSpace(h))
		v = strings.TrimPrefix(v, "http://")
		v = strings.TrimPrefix(v, "https://")
		v = strings.Trim(v, "/")
		if v == "" {
			continue
		}
		if i := strings.Index(v, ":"); i >= 0 {
			v = v[:i]
		}
		out[v] = struct{}{}
	}
	if len(out) == 0 {
		return defaultAllowedHosts
	}
	return out
}

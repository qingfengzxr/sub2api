// Package claudehttp implements the measured Claude Code 2.1.301 HTTP/1.1
// header layout. This is a versioned wire format, not client authentication.
package claudehttp

import (
	"context"
	"net/http"
	"strings"
)

const ProfileID = "claude-cli-2.1.301-h1"

type profileKey struct{}

// WithProfile must only be called for an admitted Anthropic request, after
// final header overrides. Unknown CLI versions keep the existing transport.
func WithProfile(req *http.Request) *http.Request {
	if req == nil || req.URL == nil || req.Method != http.MethodPost ||
		(!strings.HasSuffix(req.URL.Path, "/v1/messages") && !strings.HasSuffix(req.URL.Path, "/v1/messages/count_tokens")) {
		return req
	}
	var ua string
	for key, values := range req.Header {
		if strings.EqualFold(key, "User-Agent") && len(values) == 1 {
			ua = values[0]
		}
	}
	if ua != "claude-cli/2.1.301" && !(strings.HasPrefix(ua, "claude-cli/2.1.301 (") && strings.HasSuffix(ua, ")")) {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), profileKey{}, ProfileID))
}

func Enabled(req *http.Request) bool {
	return req != nil && req.Context().Value(profileKey{}) == ProfileID
}

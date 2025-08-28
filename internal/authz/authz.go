package authz

import (
	"errors"
	"os"
	"strings"
	"sync"
)

func IsReadOnly() bool { return os.Getenv("MCP_K8S_READONLY") == "true" }

func parseCSV(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func IsNamespaceAllowed(ns string) bool {
	if ns == "" {
		return true
	}
	allow := parseCSV(os.Getenv("MCP_K8S_NAMESPACE_ALLOWLIST"))
	if len(allow) == 0 {
		return true
	}
	for _, a := range allow {
		if a == ns {
			return true
		}
	}
	return false
}

func IsKindAllowed(kind string) bool {
	if kind == "" {
		return true
	}
	allow := parseCSV(os.Getenv("MCP_K8S_KIND_ALLOWLIST"))
	if len(allow) == 0 {
		return true
	}
	for _, a := range allow {
		if a == kind {
			return true
		}
	}
	return false
}

type GuardError struct {
	Code    string
	Message string
}

func (e *GuardError) Error() string { return e.Message }

func EnforceMutating(tool string, ns string, kind string) error {
	if IsReadOnly() {
		return &GuardError{Code: "READ_ONLY_BLOCKED", Message: tool + " is blocked in read-only mode"}
	}
	if !IsNamespaceAllowed(ns) {
		return &GuardError{Code: "NS_NOT_ALLOWED", Message: "Namespace " + ns + " is not in allowlist"}
	}
	if !IsKindAllowed(kind) {
		return &GuardError{Code: "KIND_NOT_ALLOWED", Message: "Kind " + kind + " is not in allowlist"}
	}
	return nil
}

// Token bucket rate limiter per tool
type tokenBucket struct {
	capacity     int
	refillPerSec int
	tokens       int
	lastRefill   int64 // epoch seconds; simplified for our usage
}

var (
	buckets = map[string]*tokenBucket{}
	mu      sync.Mutex
)

func RateLimit(tool string, burst, rate int) error {
	mu.Lock()
	defer mu.Unlock()
	b := buckets[tool]
	if b == nil {
		b = &tokenBucket{capacity: burst, refillPerSec: rate, tokens: burst}
		buckets[tool] = b
	}
	if b.tokens <= 0 {
		return errors.New("rate limit exceeded")
	}
	b.tokens--
	return nil
}

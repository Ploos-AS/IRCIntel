package probe

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Identity struct {
	Nick     string
	Username string
	Realname string
	Contact  string
}

type Policy struct {
	AllowHosts  []string
	DenyHosts   []string
	MinInterval time.Duration
}

type Runner struct {
	identity Identity
	policy   Policy
	mu       sync.Mutex
	last     map[string]time.Time
	now      func() time.Time
}

func NewRunner(identity Identity, policy Policy) (*Runner, error) {
	if strings.TrimSpace(identity.Nick) == "" { return nil, errors.New("probe nick is required") }
	if strings.TrimSpace(identity.Username) == "" { return nil, errors.New("probe username is required") }
	if strings.TrimSpace(identity.Contact) == "" { return nil, errors.New("probe contact is required") }
	if len(policy.AllowHosts) == 0 { return nil, errors.New("target allowlist is required") }
	if policy.MinInterval <= 0 { policy.MinInterval = 5 * time.Minute }
	return &Runner{identity: identity, policy: policy, last: map[string]time.Time{}, now: time.Now}, nil
}

func (r *Runner) authorize(host string) error {
	return r.authorizeKey(host, strings.ToLower(strings.TrimSpace(host)))
}

func (r *Runner) authorizeEndpoint(host, port string, tls bool) error {
	hostKey := strings.ToLower(strings.TrimSpace(host))
	port = strings.TrimSpace(port)
	if port == "" {
		if tls { port = "6697" } else { port = "6667" }
	}
	key := fmt.Sprintf("%s|%s|%t", hostKey, port, tls)
	return r.authorizeKey(host, key)
}

func (r *Runner) authorizeKey(host, rateKey string) error {
	for _, denied := range r.policy.DenyHosts {
		if hostMatch(host, denied) {
			return probeError(CodeTargetDenied, "policy", fmt.Errorf("target %q is denied", host))
		}
	}
	allowed := false
	for _, candidate := range r.policy.AllowHosts {
		if hostMatch(host, candidate) { allowed = true; break }
	}
	if !allowed {
		return probeError(CodeTargetNotAllowed, "policy", fmt.Errorf("target %q is not allowlisted", host))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if previous, ok := r.last[rateKey]; ok && now.Sub(previous) < r.policy.MinInterval {
		return probeError(CodeRateLimited, "policy", fmt.Errorf("target %q is rate limited", host))
	}
	r.last[rateKey] = now
	return nil
}

func hostMatch(host, rule string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	rule = strings.ToLower(strings.TrimSpace(rule))
	if host == rule { return true }
	if strings.HasPrefix(rule, "*.") {
		return strings.HasSuffix(host, strings.TrimPrefix(rule, "*"))
	}
	return false
}

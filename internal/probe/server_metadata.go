package probe

import (
	"sort"
	"strings"
)

// ServerMetadata captures public metadata announced during IRC registration.
type ServerMetadata struct {
	Network        string            `json:"network,omitempty"`
	Software       string            `json:"software,omitempty"`
	SoftwareVersion string           `json:"software_version,omitempty"`
	ISupport       map[string]string `json:"isupport,omitempty"`
}

func parseISupport(parts []string, metadata *ServerMetadata) {
	if len(parts) < 4 || parts[1] != "005" {
		return
	}
	if metadata.ISupport == nil {
		metadata.ISupport = map[string]string{}
	}
	for _, token := range parts[3:] {
		if strings.HasPrefix(token, ":") {
			break
		}
		key, value, ok := strings.Cut(token, "=")
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !ok {
			value = ""
		}
		metadata.ISupport[key] = value
		if strings.EqualFold(key, "NETWORK") {
			metadata.Network = value
		}
	}
}

func parseServerSoftware(parts []string, metadata *ServerMetadata) {
	if len(parts) < 4 || parts[1] != "004" {
		return
	}
	// RFC-style 004 is: <server> <version> <available user modes> <available channel modes>.
	metadata.Software = parts[2]
	metadata.SoftwareVersion = parts[3]
}

func sortedISupportKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

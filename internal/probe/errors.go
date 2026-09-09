package probe

import (
	"errors"
	"fmt"
)

const (
	CodeInvalidConfig          = "invalid_config"
	CodeDNSLookupFailed        = "dns_lookup_failed"
	CodeNoIPv4Address          = "no_ipv4_address"
	CodeNoIPv6Address          = "no_ipv6_address"
	CodeNoAddress              = "no_address"
	CodeTCPConnectFailed       = "tcp_connect_failed"
	CodeTLSHandshakeFailed     = "tls_handshake_failed"
	CodeIRCRegistrationWrite   = "irc_registration_write_failed"
	CodeIRCPongWrite           = "irc_pong_write_failed"
	CodeIRCCapEndWrite         = "irc_cap_end_write_failed"
	CodeIRCReadFailed          = "irc_read_failed"
	CodeIRCRegistrationFailed  = "irc_registration_failed"
)

type ProbeError struct {
	Code  string
	Stage string
	Err   error
}

func (e *ProbeError) Error() string {
	if e.Err == nil { return e.Code }
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *ProbeError) Unwrap() error { return e.Err }

func probeError(code, stage string, err error) error {
	return &ProbeError{Code: code, Stage: stage, Err: err}
}

func ErrorInfo(err error) (code, stage string) {
	var pe *ProbeError
	if errors.As(err, &pe) { return pe.Code, pe.Stage }
	return "unknown", "unknown"
}

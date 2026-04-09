package relay

import "errors"

var (
	ErrNoAvailableChannel = errors.New("relay: no available channel")
	ErrChannelUnavailable  = errors.New("relay: channel unavailable")
	ErrRateLimitExceeded  = errors.New("relay: rate limit exceeded")
	ErrInsufficientQuota  = errors.New("relay: insufficient quota")
	ErrCircuitOpen        = errors.New("relay: circuit open")
	ErrInvalidRequest     = errors.New("relay: invalid request")
	ErrTimeout            = errors.New("relay: timeout")
	ErrUpstreamError      = errors.New("relay: upstream error")
	ErrBillingFailed      = errors.New("relay: billing failed")
)

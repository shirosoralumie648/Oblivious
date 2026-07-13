package outboundpolicy

import (
	"errors"
	"fmt"
	"io"
)

var ErrResponseTooLarge = errors.New("outbound response body too large")

func ReadBodyLimited(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%w: limit must be positive", ErrResponseTooLarge)
	}
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: limit=%d", ErrResponseTooLarge, limit)
	}
	return body, nil
}

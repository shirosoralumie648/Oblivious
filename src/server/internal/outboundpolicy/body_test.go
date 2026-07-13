package outboundpolicy

import (
	"errors"
	"strings"
	"testing"
)

func TestReadBodyLimitedRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	_, err := ReadBodyLimited(strings.NewReader("12345"), 4)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("ReadBodyLimited error = %v, want ErrResponseTooLarge", err)
	}
}

func TestReadBodyLimitedReturnsBodyWithinLimit(t *testing.T) {
	t.Parallel()

	body, err := ReadBodyLimited(strings.NewReader("1234"), 4)
	if err != nil {
		t.Fatalf("ReadBodyLimited returned error: %v", err)
	}
	if string(body) != "1234" {
		t.Fatalf("ReadBodyLimited body = %q, want 1234", string(body))
	}
}

package handler

import (
	"net/http"
	"testing"
)

func TestProviderResponseFromHTTPAttachesUsage(t *testing.T) {
	body := []byte(`{"id":"chatcmpl_1","usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`)

	resp := providerResponseFromHTTP(http.StatusOK, body)

	if resp.Usage == nil {
		t.Fatal("expected usage to be parsed")
	}
	if resp.Usage.PromptTokens != 12 || resp.Usage.CompletionTokens != 8 || resp.Usage.TotalTokens != 20 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}

func TestProviderResponseFromHTTPLeavesMissingUsageNil(t *testing.T) {
	resp := providerResponseFromHTTP(http.StatusOK, []byte(`{"id":"img_1"}`))

	if resp.Usage != nil {
		t.Fatalf("expected nil usage when provider body has no usage, got %+v", resp.Usage)
	}
}

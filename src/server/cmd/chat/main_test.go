package main

import (
	"os"
	"strings"
	"testing"
)

func TestChatCommandUsesCommercialHTTPRouter(t *testing.T) {
	sourceBytes, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	source := string(sourceBytes)

	if !strings.Contains(source, "serverhttp.NewChatRouter(cfg, db)") {
		t.Fatal("expected chat command to serve the commercial HTTP router")
	}
	if !strings.Contains(source, "fmt.Sprintf(\":%d\", cfg.Port)") {
		t.Fatal("expected chat command to bind the configured SERVER_PORT")
	}
	for _, forbidden := range []string{"gin.Default", "demoReplyGenerator", "Demo reply", "noopUsageRecorder"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("chat command must not retain demo-only runtime code %q", forbidden)
		}
	}
}

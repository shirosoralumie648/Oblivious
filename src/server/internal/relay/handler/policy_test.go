package handler

import (
	"testing"
)

func TestAllRegisteredRoutesHaveCommercialPolicy(t *testing.T) {
	routes := getOpenAIRoutes()
	if len(routes) == 0 {
		t.Fatal("expected registered relay routes")
	}

	seen := make(map[string]bool, len(routes))
	for _, route := range routes {
		key := route.Method + " " + route.Path
		if seen[key] {
			t.Fatalf("duplicate registered route %s", key)
		}
		seen[key] = true

		policy, ok := PolicyForRoute(route.Method, route.Path)
		if !ok {
			t.Fatalf("missing commercial policy for %s", key)
		}
		if policy.Method != route.Method || policy.Path != route.Path {
			t.Fatalf("policy route mismatch for %s: got %s %s", key, policy.Method, policy.Path)
		}
		if policy.APIType != route.APIType {
			t.Fatalf("policy API type mismatch for %s: got %s want %s", key, policy.APIType.String(), route.APIType.String())
		}
		if policy.Strategy != route.Strategy {
			t.Fatalf("policy strategy mismatch for %s", key)
		}
		if policy.Class == "" {
			t.Fatalf("missing commercial class for %s", key)
		}
		if policy.Class == DisabledInProduction {
			if policy.ProductionEnabled {
				t.Fatalf("disabled route %s must not be production enabled", key)
			}
			if policy.DisabledReason == "" {
				t.Fatalf("disabled route %s needs disabled reason", key)
			}
			if policy.FutureOwner == "" {
				t.Fatalf("disabled route %s needs future owner", key)
			}
		}
	}
}

func TestPolicyRegistryDoesNotContainUnknownRegisteredRoutes(t *testing.T) {
	registered := make(map[string]bool)
	for _, route := range getOpenAIRoutes() {
		registered[route.Method+" "+route.Path] = true
	}

	policies := AllRoutePolicies()
	if len(policies) == 0 {
		t.Fatal("expected route policies")
	}

	seen := make(map[string]bool, len(policies))
	for _, policy := range policies {
		key := policy.Method + " " + policy.Path
		if seen[key] {
			t.Fatalf("duplicate policy for %s", key)
		}
		seen[key] = true
		if !registered[key] {
			t.Fatalf("policy references unregistered route %s", key)
		}
	}
}

func TestInitialCommercialPolicyClassifiesCurrentSurface(t *testing.T) {
	supported := []string{
		"POST /v1/chat/completions",
		"POST /v1/responses",
		"POST /v1/embeddings",
		"POST /v1/images/generations",
		"POST /v1/images/edits",
		"POST /v1/images/variations",
		"POST /v1/audio/speech",
		"POST /v1/audio/transcriptions",
		"POST /v1/audio/translations",
		"POST /v1/moderations",
		"POST /v1/completions",
	}
	for _, key := range supported {
		policy := mustPolicy(t, key)
		if policy.Class != CommercialSupportedBilled {
			t.Fatalf("%s class = %s, want %s", key, policy.Class, CommercialSupportedBilled)
		}
		if !policy.ProductionEnabled {
			t.Fatalf("%s should be production enabled", key)
		}
	}

	disabled := []string{
		"GET /v1/realtime",
		"POST /v1/videos",
		"POST /v1/batch",
		"GET /v1/batches",
		"GET /v1/batches/:id",
		"POST /v1/files",
		"GET /v1/files",
		"GET /v1/files/:id",
		"DELETE /v1/files/:id",
		"GET /v1/files/:id/content",
		"POST /v1/fine_tuning/jobs",
		"GET /v1/fine_tuning/jobs",
		"GET /v1/fine_tuning/jobs/:id",
		"POST /v1/fine_tuning/jobs/:id/cancel",
		"GET /v1/fine_tuning/jobs/:id/events",
		"POST /v1/assistants",
		"GET /v1/assistants",
		"GET /v1/assistants/:id",
		"POST /v1/threads",
		"GET /v1/threads/:id",
		"POST /v1/threads/:id/runs",
		"GET /v1/threads/:id/runs/:rid",
		"POST /v1/threads/:id/runs/:rid/submit",
	}
	for _, key := range disabled {
		policy := mustPolicy(t, key)
		if policy.Class != DisabledInProduction {
			t.Fatalf("%s class = %s, want %s", key, policy.Class, DisabledInProduction)
		}
		if policy.ProductionEnabled {
			t.Fatalf("%s should be disabled in production", key)
		}
	}
}

func TestSupportedRoutePoliciesDeclareCostAbuseGuardrails(t *testing.T) {
	for _, policy := range AllRoutePolicies() {
		if policy.Class != CommercialSupportedBilled {
			continue
		}
		key := policy.Method + " " + policy.Path
		if policy.AuthPolicy == "" {
			t.Fatalf("%s missing auth policy", key)
		}
		if !policy.TenantIdentityRequired {
			t.Fatalf("%s must require tenant identity", key)
		}
		if policy.RateLimitPolicy == "" {
			t.Fatalf("%s missing rate-limit policy", key)
		}
		if policy.AuditPolicy == "" {
			t.Fatalf("%s missing audit policy", key)
		}
	}
}

func mustPolicy(t *testing.T, routeKey string) RoutePolicy {
	t.Helper()
	method, path, ok := splitRouteKey(routeKey)
	if !ok {
		t.Fatalf("bad route key %q", routeKey)
	}
	policy, ok := PolicyForRoute(method, path)
	if !ok {
		t.Fatalf("missing policy for %s", routeKey)
	}
	return policy
}

func splitRouteKey(key string) (string, string, bool) {
	for i := 0; i < len(key); i++ {
		if key[i] == ' ' {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}

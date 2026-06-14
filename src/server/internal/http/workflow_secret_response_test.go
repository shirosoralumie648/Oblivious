package http

import (
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"strings"
	"testing"
)

func TestWorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "workflow-secret-redaction@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)

	secrets := []string{
		"whsec-top-level-secret",
		"whsec-camel-secret",
		"node-nested-secret",
		"trigger-nested-secret",
	}

	createBody := commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/workflows", `{
		"name": "Secret Workflow",
		"status": "published",
		"definition": {
			"webhook_secret": "whsec-top-level-secret",
			"webhookSecret": "whsec-camel-secret",
			"nodes": [{
				"id": "start",
				"type": "manual",
				"input": {
					"message": "tenant scoped",
					"secret": "node-nested-secret"
				}
			}],
			"edges": [],
			"triggers": {
				"webhook": [{
					"id": "incoming",
					"secret": "trigger-nested-secret"
				}]
			}
		},
		"variables": {"publicLabel": "visible"}
	}`, cookie, csrfToken, stdhttp.StatusCreated)
	assertWorkflowResponseRedactsSecrets(t, string(createBody), secrets...)
	assertWorkflowResponseContainsRedactedMarker(t, string(createBody))

	var createResponse struct {
		Data struct {
			ID         string         `json:"id"`
			Definition map[string]any `json:"definition"`
			Variables  map[string]any `json:"variables"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createBody, &createResponse); err != nil {
		t.Fatalf("decode workflow create response: %v", err)
	}
	workflowID := createResponse.Data.ID
	if workflowID == "" {
		t.Fatalf("expected created workflow ID, got %s", string(createBody))
	}
	assertWorkflowDefinitionSecretsRedacted(t, createResponse.Data.Definition)
	if createResponse.Data.Variables["publicLabel"] != "visible" {
		t.Fatalf("expected non-secret variables to be preserved, got %+v", createResponse.Data.Variables)
	}
	assertWorkflowSQLContainsSecrets(t, database, "workflows", "definition", "id = $1 AND organization_id = $2", []any{workflowID, organizationID}, secrets...)
	assertWorkflowSQLContainsSecrets(t, database, "workflow_versions", "definition", "workflow_id = $1 AND organization_id = $2 AND version = 1", []any{workflowID, organizationID}, secrets...)

	listBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/workflows", "", cookie, "", stdhttp.StatusOK)
	assertWorkflowResponseRedactsSecrets(t, string(listBody), secrets...)
	assertWorkflowResponseContainsRedactedMarker(t, string(listBody))

	getBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/workflows/"+workflowID, "", cookie, "", stdhttp.StatusOK)
	assertWorkflowResponseRedactsSecrets(t, string(getBody), secrets...)
	assertWorkflowResponseContainsRedactedMarker(t, string(getBody))

	updateBody := commercialDoJSON(t, router, stdhttp.MethodPut, "/api/v1/workflows/"+workflowID, `{
		"name": "Secret Workflow Updated",
		"status": "published",
		"definition": {
			"webhook_secret": "********",
			"webhookSecret": "********",
			"nodes": [{
				"id": "start",
				"type": "manual",
				"input": {
					"message": "updated without replacing secrets",
					"secret": "********"
				}
			}],
			"edges": [],
			"triggers": {
				"webhook": [{
					"id": "incoming",
					"secret": "********"
				}]
			}
		},
		"variables": {"publicLabel": "visible-updated"}
	}`, cookie, csrfToken, stdhttp.StatusOK)
	assertWorkflowResponseRedactsSecrets(t, string(updateBody), secrets...)
	assertWorkflowResponseContainsRedactedMarker(t, string(updateBody))
	assertWorkflowSQLContainsSecrets(t, database, "workflows", "definition", "id = $1 AND organization_id = $2", []any{workflowID, organizationID}, secrets...)
	assertWorkflowSQLContainsSecrets(t, database, "workflow_versions", "definition", "workflow_id = $1 AND organization_id = $2 AND version = 2", []any{workflowID, organizationID}, secrets...)
	assertWorkflowSQLContainsText(t, database, "workflows", "definition", "id = $1 AND organization_id = $2", []any{workflowID, organizationID}, "updated without replacing secrets")

	versionsBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/workflows/"+workflowID+"/versions", "", cookie, "", stdhttp.StatusOK)
	assertWorkflowResponseRedactsSecrets(t, string(versionsBody), secrets...)
	assertWorkflowResponseContainsRedactedMarker(t, string(versionsBody))

	executionBody := commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/workflows/"+workflowID+"/execute", `{
		"input": {
			"customerId": "customer_123",
			"note": "preserve execution input"
		}
	}`, cookie, csrfToken, stdhttp.StatusCreated)
	assertWorkflowResponseRedactsSecrets(t, string(executionBody), secrets...)
	assertWorkflowResponseContainsRedactedMarker(t, string(executionBody))

	var executionResponse struct {
		Data struct {
			ID               string         `json:"id"`
			Input            map[string]any `json:"input"`
			Context          map[string]any `json:"context"`
			WorkflowSnapshot map[string]any `json:"workflowSnapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal(executionBody, &executionResponse); err != nil {
		t.Fatalf("decode workflow execution response: %v", err)
	}
	executionID := executionResponse.Data.ID
	if executionID == "" {
		t.Fatalf("expected execution ID, got %s", string(executionBody))
	}
	assertWorkflowDefinitionSecretsRedacted(t, executionResponse.Data.WorkflowSnapshot)
	if executionResponse.Data.Input["customerId"] != "customer_123" {
		t.Fatalf("expected execution input to be preserved, got %+v", executionResponse.Data.Input)
	}
	trigger, ok := executionResponse.Data.Context["trigger"].(map[string]any)
	if !ok || trigger["type"] != "manual" {
		t.Fatalf("expected execution context trigger to be preserved, got %+v", executionResponse.Data.Context)
	}
	assertWorkflowSQLContainsSecrets(t, database, "workflow_executions", "workflow_snapshot", "id = $1 AND organization_id = $2", []any{executionID, organizationID}, secrets...)

	listExecutionsBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/workflows/"+workflowID+"/executions", "", cookie, "", stdhttp.StatusOK)
	assertWorkflowResponseRedactsSecrets(t, string(listExecutionsBody), secrets...)
	assertWorkflowResponseContainsRedactedMarker(t, string(listExecutionsBody))

	getExecutionBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/workflows/"+workflowID+"/executions/"+executionID, "", cookie, "", stdhttp.StatusOK)
	assertWorkflowResponseRedactsSecrets(t, string(getExecutionBody), secrets...)
	assertWorkflowResponseContainsRedactedMarker(t, string(getExecutionBody))

	debugBody := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/workflows/"+workflowID+"/executions/"+executionID+"/debug-snapshot", "", cookie, "", stdhttp.StatusOK)
	assertWorkflowResponseRedactsSecrets(t, string(debugBody), secrets...)
	if !strings.Contains(string(debugBody), "customer_123") || !strings.Contains(string(debugBody), `"trigger"`) {
		t.Fatalf("expected debug snapshot to preserve execution input/context, got %s", string(debugBody))
	}
}

func assertWorkflowResponseRedactsSecrets(t *testing.T, body string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(body, secret) {
			t.Fatalf("expected workflow response to redact %q, got %s", secret, body)
		}
	}
}

func assertWorkflowResponseContainsRedactedMarker(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, workflowRedactedSecret) {
		t.Fatalf("expected workflow response to contain redacted marker %q, got %s", workflowRedactedSecret, body)
	}
}

func assertWorkflowDefinitionSecretsRedacted(t *testing.T, definition map[string]any) {
	t.Helper()
	if got := definition["webhook_secret"]; got != workflowRedactedSecret {
		t.Fatalf("expected top-level webhook_secret redacted, got %#v in %+v", got, definition)
	}
	if got := definition["webhookSecret"]; got != workflowRedactedSecret {
		t.Fatalf("expected top-level webhookSecret redacted, got %#v in %+v", got, definition)
	}
	nodes, ok := definition["nodes"].([]any)
	if !ok || len(nodes) != 1 {
		t.Fatalf("expected one workflow node, got %+v", definition["nodes"])
	}
	node, ok := nodes[0].(map[string]any)
	if !ok {
		t.Fatalf("expected workflow node object, got %#v", nodes[0])
	}
	input, ok := node["input"].(map[string]any)
	if !ok || input["secret"] != workflowRedactedSecret {
		t.Fatalf("expected nested node input secret redacted, got %+v", node["input"])
	}
	triggers, ok := definition["triggers"].(map[string]any)
	if !ok {
		t.Fatalf("expected triggers object, got %+v", definition["triggers"])
	}
	webhooks, ok := triggers["webhook"].([]any)
	if !ok || len(webhooks) != 1 {
		t.Fatalf("expected one webhook trigger, got %+v", triggers["webhook"])
	}
	webhook, ok := webhooks[0].(map[string]any)
	if !ok || webhook["secret"] != workflowRedactedSecret {
		t.Fatalf("expected webhook trigger secret redacted, got %+v", webhooks[0])
	}
}

func assertWorkflowSQLContainsSecrets(t *testing.T, database *sql.DB, table, column, where string, args []any, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		assertWorkflowSQLContainsText(t, database, table, column, where, args, secret)
	}
}

func assertWorkflowSQLContainsText(t *testing.T, database *sql.DB, table, column, where string, args []any, text string) {
	t.Helper()
	var payload string
	query := "SELECT " + column + "::text FROM " + table + " WHERE " + where
	if err := database.QueryRow(query, args...).Scan(&payload); err != nil {
		t.Fatalf("query %s.%s: %v", table, column, err)
	}
	if !strings.Contains(payload, text) {
		t.Fatalf("expected %s.%s to contain %q, got %s", table, column, text, payload)
	}
}

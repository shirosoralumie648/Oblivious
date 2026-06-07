package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarketplaceTemplateRoutesCreateListDetailAndInstall(t *testing.T) {
	database := testDatabase(t)
	router := NewServer(testConfig(), database).Handler

	cookie, csrfToken, _ := registerHTTPUser(t, router, "marketplace-template-publisher@example.com")

	createRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/templates", strings.NewReader(`{
		"type": "workflow",
		"name": "Lead Intake Template",
		"description": "Reusable workflow template for lead qualification.",
		"templateData": {"nodes":[{"id":"start","type":"trigger"}]},
		"category": "sales",
		"tags": ["crm","lead"]
	}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	addCSRF(createRequest, csrfToken)
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("create template expected 201, got %d with body %s", createRecorder.Code, createRecorder.Body.String())
	}

	var createResponse struct {
		Data struct {
			ID           string          `json:"id"`
			Type         string          `json:"type"`
			Name         string          `json:"name"`
			TemplateData json.RawMessage `json:"templateData"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("decode create template response: %v", err)
	}
	if createResponse.Data.ID == "" || createResponse.Data.Type != "workflow" || !strings.Contains(string(createResponse.Data.TemplateData), `"nodes"`) {
		t.Fatalf("unexpected created template response: %s", createRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/marketplace/templates?type=workflow", nil))
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list templates expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	if !strings.Contains(listRecorder.Body.String(), "Lead Intake Template") {
		t.Fatalf("expected template in list response, got %s", listRecorder.Body.String())
	}

	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/marketplace/templates/"+createResponse.Data.ID, nil))
	if detailRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("get template expected 200, got %d with body %s", detailRecorder.Code, detailRecorder.Body.String())
	}

	installRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/templates/"+createResponse.Data.ID+"/install", nil)
	installRequest.AddCookie(cookie)
	addCSRF(installRequest, csrfToken)
	installRecorder := httptest.NewRecorder()
	router.ServeHTTP(installRecorder, installRequest)
	if installRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("install template expected 201, got %d with body %s", installRecorder.Code, installRecorder.Body.String())
	}
	if !strings.Contains(installRecorder.Body.String(), `"templateID":"`+createResponse.Data.ID+`"`) || !strings.Contains(installRecorder.Body.String(), `"templateData"`) {
		t.Fatalf("expected install response with template data, got %s", installRecorder.Body.String())
	}
}

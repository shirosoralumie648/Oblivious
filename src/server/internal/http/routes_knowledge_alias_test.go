package http

import (
	"context"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/knowledge"
)

func TestRegisterKnowledgeAliasRoutesDispatchesRootCollection(t *testing.T) {
	store := &knowledgeFakeStore{
		createdBase: knowledge.KnowledgeBase{ID: "kb_created", Name: "Specs"},
		listBases: []knowledge.KnowledgeBase{
			{ID: "kb_1", Name: "Runbooks", UpdatedAt: time.Date(2026, time.April, 3, 9, 0, 0, 0, time.UTC)},
		},
	}
	handler := newKnowledgeTestHandler(store)
	mux := stdhttp.NewServeMux()
	authMiddleware := &recordingSessionMiddleware{}
	registerKnowledgeAliasRoutes(mux, authMiddleware, handler)

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, knowledgeAliasRequest(stdhttp.MethodGet, "/api/v1/knowledge-bases", ""))
	if list.Code != stdhttp.StatusOK {
		t.Fatalf("GET expected 200, got %d with body %s", list.Code, list.Body.String())
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected list to use session org_1, got %q", store.organizationID)
	}

	create := httptest.NewRecorder()
	mux.ServeHTTP(create, knowledgeAliasRequest(stdhttp.MethodPost, "/api/v1/knowledge-bases", `{"name":"Specs"}`))
	if create.Code != stdhttp.StatusOK {
		t.Fatalf("POST expected 200, got %d with body %s", create.Code, create.Body.String())
	}
	if store.createdName != "Specs" {
		t.Fatalf("expected created name Specs, got %q", store.createdName)
	}
	if authMiddleware.requestCalls != 2 {
		t.Fatalf("expected session middleware for both collection requests, got %d", authMiddleware.requestCalls)
	}
}

func TestRegisterKnowledgeAliasRoutesDispatchesRootKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{
		detailBase:  knowledge.KnowledgeBase{ID: "kb_2", Name: "Details"},
		updatedBase: knowledge.KnowledgeBase{ID: "kb_2", Name: "Updated"},
	}
	handler := newKnowledgeTestHandler(store)
	mux := stdhttp.NewServeMux()
	registerKnowledgeAliasRoutes(mux, &recordingSessionMiddleware{}, handler)

	get := httptest.NewRecorder()
	mux.ServeHTTP(get, knowledgeAliasRequest(stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_2", ""))
	if get.Code != stdhttp.StatusOK {
		t.Fatalf("GET expected 200, got %d with body %s", get.Code, get.Body.String())
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected get to request kb_2, got %q", store.requestedID)
	}

	update := httptest.NewRecorder()
	mux.ServeHTTP(update, knowledgeAliasRequest(stdhttp.MethodPut, "/api/v1/knowledge-bases/kb_2", `{"name":"Updated"}`))
	if update.Code != stdhttp.StatusOK {
		t.Fatalf("PUT expected 200, got %d with body %s", update.Code, update.Body.String())
	}
	if store.requestedID != "kb_2" || store.createdName != "Updated" {
		t.Fatalf("expected update kb_2/Updated, got id=%q name=%q", store.requestedID, store.createdName)
	}

	remove := httptest.NewRecorder()
	mux.ServeHTTP(remove, knowledgeAliasRequest(stdhttp.MethodDelete, "/api/v1/knowledge-bases/kb_2", ""))
	if remove.Code != stdhttp.StatusNoContent {
		t.Fatalf("DELETE expected 204, got %d with body %s", remove.Code, remove.Body.String())
	}
	if store.deletedID != "kb_2" {
		t.Fatalf("expected deleted kb_2, got %q", store.deletedID)
	}
}

func TestRegisterKnowledgeAliasRoutesDispatchesDocumentsAndRetrieve(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc:      knowledge.KnowledgeDocument{ID: "doc_1", Title: "Plan"},
		createdTestCase: knowledge.KnowledgeRetrievalTestCase{ID: "krtc_1", KnowledgeBaseID: "kb_2", Query: "deployment"},
		detailBase:      knowledge.KnowledgeBase{ID: "kb_2", RetrievalLimit: 3, RetrievalMode: knowledge.KnowledgeRetrievalModeHybrid},
		documents:       []knowledge.KnowledgeDocument{{ID: "doc_1", Title: "Plan"}},
		listTestCases: []knowledge.KnowledgeRetrievalTestCase{{
			ID:                 "krtc_1",
			KnowledgeBaseID:    "kb_2",
			Query:              "deployment",
			ExpectedDocumentID: "doc_1",
			ExpectedChunkID:    "chunk_1",
		}},
		retrievalResults: []knowledge.KnowledgeRetrievalResult{{
			DocumentID: "doc_1",
			ChunkID:    "chunk_1",
			Snippet:    "deployment rollback",
		}},
	}
	handler := newKnowledgeTestHandler(store)
	mux := stdhttp.NewServeMux()
	registerKnowledgeAliasRoutes(mux, &recordingSessionMiddleware{}, handler)

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, knowledgeAliasRequest(stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_2/documents", ""))
	if list.Code != stdhttp.StatusOK {
		t.Fatalf("documents GET expected 200, got %d with body %s", list.Code, list.Body.String())
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected list documents kb_2, got %q", store.requestedID)
	}

	create := httptest.NewRecorder()
	mux.ServeHTTP(create, knowledgeAliasRequest(stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_2/documents", `{"title":"Plan","content":"Initial plan"}`))
	if create.Code != stdhttp.StatusOK {
		t.Fatalf("documents POST expected 200, got %d with body %s", create.Code, create.Body.String())
	}
	if store.requestedID != "kb_2" || store.requestedDoc.Title != "Plan" {
		t.Fatalf("expected create document kb_2/Plan, got id=%q doc=%+v", store.requestedID, store.requestedDoc)
	}

	retrieve := httptest.NewRecorder()
	mux.ServeHTTP(retrieve, knowledgeAliasRequest(stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_2/retrieve", `{"query":"deployment"}`))
	if retrieve.Code != stdhttp.StatusOK {
		t.Fatalf("retrieve expected 200, got %d with body %s", retrieve.Code, retrieve.Body.String())
	}
	if store.requestedID != "kb_2" || store.retrievalQuery != "deployment" {
		t.Fatalf("expected retrieve kb_2/deployment, got id=%q query=%q", store.requestedID, store.retrievalQuery)
	}

	testCase := httptest.NewRecorder()
	mux.ServeHTTP(testCase, knowledgeAliasRequest(stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_2/retrieval-test-cases", `{
		"query":"deployment",
		"expectedResult":{"documentId":"doc_1","documentTitle":"Plan","chunkId":"kdc_1","chunkIndex":0,"snippet":"deployment"}
	}`))
	if testCase.Code != stdhttp.StatusCreated {
		t.Fatalf("retrieval test case expected 201, got %d with body %s", testCase.Code, testCase.Body.String())
	}
	if store.requestedID != "kb_2" || store.testCaseRequest.ExpectedResult.ChunkID != "kdc_1" {
		t.Fatalf("expected retrieval test case kb_2/kdc_1, got id=%q req=%+v", store.requestedID, store.testCaseRequest)
	}

	listCases := httptest.NewRecorder()
	mux.ServeHTTP(listCases, knowledgeAliasRequest(stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_2/retrieval-test-cases", ""))
	if listCases.Code != stdhttp.StatusOK {
		t.Fatalf("retrieval test case list expected 200, got %d with body %s", listCases.Code, listCases.Body.String())
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected list retrieval test cases kb_2, got id=%q", store.requestedID)
	}

	runCases := httptest.NewRecorder()
	mux.ServeHTTP(runCases, knowledgeAliasRequest(stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_2/retrieval-test-cases/run", `{"mode":"hybrid","limit":3}`))
	if runCases.Code != stdhttp.StatusOK {
		t.Fatalf("retrieval test case run expected 200, got %d with body %s", runCases.Code, runCases.Body.String())
	}
	if store.retrievalOptions.Mode != knowledge.KnowledgeRetrievalModeHybrid || store.retrievalOptions.Limit != 3 {
		t.Fatalf("expected hybrid retrieval test run options, got %+v", store.retrievalOptions)
	}
}

func TestRegisterKnowledgeAliasRoutesDispatchesDocumentUpload(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc: knowledge.KnowledgeDocument{ID: "doc_upload", Title: "Runbook.txt"},
	}
	handler := newKnowledgeTestHandler(store)
	mux := stdhttp.NewServeMux()
	registerKnowledgeAliasRoutes(mux, &recordingSessionMiddleware{}, handler)
	body, contentType := knowledgeUploadMultipartBody(t, nil, "file", "Runbook.txt", "text/plain", "deploy rollback")
	request := knowledgeAliasRequest(stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_2/documents/upload", "")
	request.Body = io.NopCloser(body)
	request.ContentLength = int64(body.Len())
	request.Header.Set("Content-Type", contentType)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusAccepted {
		t.Fatalf("upload expected 202, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(store.createdIngestionJobs) != 1 {
		t.Fatalf("expected one ingestion job, got %+v", store.createdIngestionJobs)
	}
	if store.createdIngestionJobs[0].KnowledgeBaseID != "kb_2" || store.createdIngestionJobs[0].Title != "Runbook.txt" {
		t.Fatalf("expected upload kb_2/Runbook.txt, got %+v", store.createdIngestionJobs[0])
	}
}

func TestRegisterKnowledgeAliasRoutesDispatchesRootDocumentDelete(t *testing.T) {
	store := &knowledgeFakeStore{}
	handler := newKnowledgeTestHandler(store)
	mux := stdhttp.NewServeMux()
	authMiddleware := &recordingSessionMiddleware{}
	registerKnowledgeAliasRoutes(mux, authMiddleware, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, knowledgeAliasRequest(stdhttp.MethodDelete, "/api/v1/documents/doc_1", ""))

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.deletedDocID != "doc_1" {
		t.Fatalf("expected root document delete to delete doc_1, got %q", store.deletedDocID)
	}
	if store.requestedID != "" {
		t.Fatalf("expected root document delete not to require KB id, got %q", store.requestedID)
	}
	if authMiddleware.requestCalls != 1 {
		t.Fatalf("expected root document delete to require session, got %d calls", authMiddleware.requestCalls)
	}
}

func knowledgeAliasRequest(method, path, body string) *stdhttp.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		User:           auth.User{ID: "user_1"},
		WorkspaceID:    "workspace_1",
		OrganizationID: "org_1",
	}))
}

package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/auth"
)

func TestAdminPricingCatalogHandlerCreateApproveRejectRollbackAndList(t *testing.T) {
	store := &pricingCatalogHTTPStore{
		active: []admin.RelayPricingCatalogEntry{{
			ID:        "rpe_existing_prompt",
			APIType:   "chat",
			Model:     "gpt-4o",
			Dimension: "prompt_tokens",
			UnitCost:  0.002,
			Markup:    1,
			Currency:  "quota",
			Source:    "litellm",
			Active:    true,
		}},
		imports: map[string]*admin.RelayPricingCatalogImport{
			"rpci_original": {
				ID:       "rpci_original",
				Provider: "openai",
				Source:   "litellm",
				Status:   "approved",
				Diff: admin.RelayPricingCatalogDiff{Entries: []admin.RelayPricingCatalogDiffEntry{{
					Action: "update",
					Key:    "chat/gpt-4o/prompt_tokens",
					Before: &admin.RelayPricingCatalogEntry{
						ID:        "rpe_old_prompt",
						APIType:   "chat",
						Model:     "gpt-4o",
						Dimension: "prompt_tokens",
						UnitCost:  0.001,
						Markup:    1,
						Currency:  "quota",
						Source:    "litellm",
						Active:    false,
					},
					After: &admin.RelayPricingCatalogEntry{
						ID:        "rpe_existing_prompt",
						APIType:   "chat",
						Model:     "gpt-4o",
						Dimension: "prompt_tokens",
						UnitCost:  0.002,
						Markup:    1,
						Currency:  "quota",
						Source:    "litellm",
						Active:    true,
					},
				}}},
			},
		},
	}
	handler := newAdminHandler(admin.NewService(store))
	session := adminPricingCatalogTestSession()

	createRequest := adminPricingCatalogRequest(stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/imports", `{
		"provider": "OpenAI",
		"source": "litellm",
		"entries": [{
			"apiType": "chat",
			"model": "gpt-4o",
			"dimension": "prompt_tokens",
			"unitCost": 0.003
		}]
	}`, session)
	createRequest.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.2")
	createRecorder := httptest.NewRecorder()
	handler.createRelayPricingCatalogImport(createRecorder, createRequest)
	if createRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	created := adminPricingCatalogDecodeImport(t, createRecorder)
	if created.ID == "" || created.Provider != "openai" || created.Status != "pending" {
		t.Fatalf("expected normalized pending import, got %+v", created)
	}
	if store.created == nil || store.created.ImportedBy != session.User.ID || store.created.ImportedByEmail != session.User.Email {
		t.Fatalf("expected create to persist actor evidence, got %+v", store.created)
	}
	if len(store.auditEntries) == 0 || store.auditEntries[len(store.auditEntries)-1].Action != "pricing.relay_catalog.import.create" ||
		store.auditEntries[len(store.auditEntries)-1].IPAddress != "198.51.100.2" {
		t.Fatalf("expected create audit with client ip, got %+v", store.auditEntries)
	}

	listRequest := adminPricingCatalogRequest(stdhttp.MethodGet, "/api/v1/admin/pricing/relay-catalog/imports?provider=OpenAI&status=pending&limit=1", "", session)
	listRecorder := httptest.NewRecorder()
	handler.listRelayPricingCatalogImports(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	if store.importFilter.Provider != "openai" || store.importFilter.Status != "pending" || store.importFilter.Limit != 1 {
		t.Fatalf("expected normalized list filter, got %+v", store.importFilter)
	}

	approveRecorder := httptest.NewRecorder()
	handler.approveRelayPricingCatalogImport(approveRecorder, adminPricingCatalogRequest(stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/imports/"+created.ID+"/approve", "", session), created.ID)
	if approveRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected approve 200, got %d: %s", approveRecorder.Code, approveRecorder.Body.String())
	}
	if store.approvedImport == nil || store.approvedImport.ApprovedBy != session.User.ID {
		t.Fatalf("expected approve to persist actor evidence, got %+v", store.approvedImport)
	}

	reject := &admin.RelayPricingCatalogImport{ID: "rpci_reject", Provider: "openai", Source: "litellm", Status: "pending"}
	store.imports[reject.ID] = reject
	rejectRecorder := httptest.NewRecorder()
	handler.rejectRelayPricingCatalogImport(rejectRecorder, adminPricingCatalogRequest(stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/imports/rpci_reject/reject", `{"reason":"bad source hash"}`, session), "rpci_reject")
	if rejectRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected reject 200, got %d: %s", rejectRecorder.Code, rejectRecorder.Body.String())
	}
	if store.rejectedImport == nil || store.rejectedReason != "bad source hash" {
		t.Fatalf("expected reject reason to persist, got import=%+v reason=%q", store.rejectedImport, store.rejectedReason)
	}

	rollbackRecorder := httptest.NewRecorder()
	handler.rollbackRelayPricingCatalogImport(rollbackRecorder, adminPricingCatalogRequest(stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/imports/rpci_original/rollback", `{"notes":"restore previous catalog"}`, session), "rpci_original")
	if rollbackRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected rollback 201, got %d: %s", rollbackRecorder.Code, rollbackRecorder.Body.String())
	}
	rollback := adminPricingCatalogDecodeImport(t, rollbackRecorder)
	if rollback.Status != "pending" || rollback.Source != "rollback:rpci_original" || len(rollback.Entries) == 0 {
		t.Fatalf("expected pending rollback import with restored entries, got %+v", rollback)
	}
}

func TestAdminPricingCatalogHandlerListsSyncRuns(t *testing.T) {
	store := &pricingCatalogHTTPStore{
		syncRuns: []*admin.RelayPricingCatalogSyncRun{{
			ID:       "rpcs_1",
			Job:      "manual",
			Provider: "openai",
			Source:   "litellm",
			Status:   "succeeded",
		}},
	}
	handler := newAdminHandler(admin.NewService(store))

	request := adminPricingCatalogRequest(stdhttp.MethodGet, "/api/v1/admin/pricing/relay-catalog/sync-runs?provider=OpenAI&status=succeeded&limit=5", "", adminPricingCatalogTestSession())
	recorder := httptest.NewRecorder()
	handler.listRelayPricingCatalogSyncRuns(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected sync run list 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.syncRunFilter.Provider != "openai" || store.syncRunFilter.Status != "succeeded" || store.syncRunFilter.Limit != 5 {
		t.Fatalf("expected normalized sync-run filter, got %+v", store.syncRunFilter)
	}
}

func adminPricingCatalogRequest(method, path, body string, session auth.Session) *stdhttp.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	return httptest.NewRequest(method, path, reader).
		WithContext(context.WithValue(context.Background(), sessionContextKey, session))
}

func adminPricingCatalogDecodeImport(t *testing.T, recorder *httptest.ResponseRecorder) admin.RelayPricingCatalogImport {
	t.Helper()
	var response struct {
		Data admin.RelayPricingCatalogImport `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode relay pricing catalog import response: %v", err)
	}
	return response.Data
}

func adminPricingCatalogTestSession() auth.Session {
	return auth.Session{
		ID: "session_admin",
		User: auth.User{
			ID:    "user_admin",
			Email: "admin@example.com",
			Name:  "Admin",
			Role:  "admin",
		},
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		ExpiresAt:      time.Now().Add(time.Hour),
	}
}

type pricingCatalogHTTPStore struct {
	admin.Store
	active         []admin.RelayPricingCatalogEntry
	imports        map[string]*admin.RelayPricingCatalogImport
	created        *admin.RelayPricingCatalogImport
	approvedImport *admin.RelayPricingCatalogImport
	rejectedImport *admin.RelayPricingCatalogImport
	rejectedReason string
	importFilter   admin.RelayPricingCatalogImportFilter
	syncRuns       []*admin.RelayPricingCatalogSyncRun
	syncRunFilter  admin.RelayPricingCatalogSyncRunFilter
	auditEntries   []*admin.AuditEntry
}

func (s *pricingCatalogHTTPStore) ListActiveRelayPricingCatalogEntries(ctx context.Context) ([]admin.RelayPricingCatalogEntry, error) {
	return append([]admin.RelayPricingCatalogEntry{}, s.active...), nil
}

func (s *pricingCatalogHTTPStore) CreateRelayPricingCatalogImport(ctx context.Context, catalogImport admin.RelayPricingCatalogImport) (*admin.RelayPricingCatalogImport, error) {
	s.created = &catalogImport
	if s.imports == nil {
		s.imports = map[string]*admin.RelayPricingCatalogImport{}
	}
	s.imports[catalogImport.ID] = &catalogImport
	return &catalogImport, nil
}

func (s *pricingCatalogHTTPStore) GetRelayPricingCatalogImport(ctx context.Context, importID string) (*admin.RelayPricingCatalogImport, error) {
	if s.imports != nil {
		if catalogImport, ok := s.imports[importID]; ok {
			return catalogImport, nil
		}
	}
	return nil, admin.ErrRelayPricingCatalogImportNotFound
}

func (s *pricingCatalogHTTPStore) ListRelayPricingCatalogImports(ctx context.Context, filter admin.RelayPricingCatalogImportFilter) ([]*admin.RelayPricingCatalogImport, int, error) {
	s.importFilter = filter
	imports := make([]*admin.RelayPricingCatalogImport, 0, len(s.imports))
	for _, catalogImport := range s.imports {
		imports = append(imports, catalogImport)
	}
	return imports, len(imports), nil
}

func (s *pricingCatalogHTTPStore) ApproveRelayPricingCatalogImport(ctx context.Context, importID, actorID, actorEmail string) (*admin.RelayPricingCatalogImport, error) {
	catalogImport, err := s.GetRelayPricingCatalogImport(ctx, importID)
	if err != nil {
		return nil, err
	}
	if catalogImport.Status != "pending" {
		return nil, admin.ErrRelayPricingCatalogImportNotPending
	}
	approved := *catalogImport
	approved.Status = "approved"
	approved.ApprovedBy = actorID
	approved.ApprovedByEmail = actorEmail
	now := time.Now().UTC()
	approved.ApprovedAt = &now
	s.imports[importID] = &approved
	s.approvedImport = &approved
	return &approved, nil
}

func (s *pricingCatalogHTTPStore) RejectRelayPricingCatalogImport(ctx context.Context, importID, actorID, actorEmail, reason string) (*admin.RelayPricingCatalogImport, error) {
	catalogImport, err := s.GetRelayPricingCatalogImport(ctx, importID)
	if err != nil {
		return nil, err
	}
	if catalogImport.Status != "pending" {
		return nil, admin.ErrRelayPricingCatalogImportNotPending
	}
	rejected := *catalogImport
	rejected.Status = "rejected"
	s.imports[importID] = &rejected
	s.rejectedImport = &rejected
	s.rejectedReason = reason
	return &rejected, nil
}

func (s *pricingCatalogHTTPStore) CreateRelayPricingCatalogSyncRun(ctx context.Context, run admin.RelayPricingCatalogSyncRun) (*admin.RelayPricingCatalogSyncRun, error) {
	s.syncRuns = append(s.syncRuns, &run)
	return &run, nil
}

func (s *pricingCatalogHTTPStore) ListRelayPricingCatalogSyncRuns(ctx context.Context, filter admin.RelayPricingCatalogSyncRunFilter) ([]*admin.RelayPricingCatalogSyncRun, int, error) {
	s.syncRunFilter = filter
	return append([]*admin.RelayPricingCatalogSyncRun{}, s.syncRuns...), len(s.syncRuns), nil
}

func (s *pricingCatalogHTTPStore) CreateAuditEntry(ctx context.Context, entry *admin.AuditEntry) error {
	s.auditEntries = append(s.auditEntries, entry)
	return nil
}

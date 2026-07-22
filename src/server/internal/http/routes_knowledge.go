package http

import (
	stdhttp "net/http"
	"strings"
)

func knowledgeRouteSurfaceOperations() []OperationContractMetadataV1 {
	return routeSurfaceOperationsFromSpecs([]routeSurfaceOperationSpec{
		{"GET", "/api/v1/app/knowledge-bases", "listKnowledgeBases", "cookie", false, "knowledge.retrieval", "", "none", "", "200", "application/json", "inline", "sha256:c32092c17bfcf1143ccaaaaad125b3e4019d9019832b8b0493b5e26bfcfa80ba"},
		{"POST", "/api/v1/app/knowledge-bases", "createKnowledgeBase", "cookie+csrf", true, "knowledge.ingestion", "application/json", "ref", "#/components/schemas/CreateKnowledgeBaseRequest", "200", "application/json", "inline", "sha256:7192a17ce5f016b6a5819ffebbf0c62a032585334993e5f431110f9b060a5543"},
		{"DELETE", "/api/v1/app/knowledge-bases/{knowledgeBaseId}", "deleteKnowledgeBase", "cookie+csrf", true, "knowledge.ingestion", "", "none", "", "204", "", "none", ""},
		{"GET", "/api/v1/app/knowledge-bases/{knowledgeBaseId}", "getKnowledgeBase", "cookie", false, "knowledge.retrieval", "", "none", "", "200", "application/json", "inline", "sha256:7192a17ce5f016b6a5819ffebbf0c62a032585334993e5f431110f9b060a5543"},
		{"PUT", "/api/v1/app/knowledge-bases/{knowledgeBaseId}", "updateKnowledgeBase", "cookie+csrf", true, "knowledge.ingestion", "application/json", "ref", "#/components/schemas/CreateKnowledgeBaseRequest", "200", "application/json", "inline", "sha256:7192a17ce5f016b6a5819ffebbf0c62a032585334993e5f431110f9b060a5543"},
		{"GET", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "listKnowledgeDocuments", "cookie", false, "knowledge.retrieval", "", "none", "", "200", "application/json", "inline", "sha256:104dbf0b9053996e76c2b18eb4a5f230334f77490939f7773df2b6f93e0b2e49"},
		{"POST", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "createKnowledgeDocument", "cookie+csrf", true, "knowledge.ingestion", "application/json", "ref", "#/components/schemas/CreateDocumentRequest", "200", "application/json", "inline", "sha256:7f200d9c163803f673f4209c7d508bd0ef35e048d035cc73101a3aa0d7b7c4d1"},
		{"GET", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/ingestion-jobs", "listKnowledgeDocumentIngestionJobs", "cookie", false, "knowledge.ingestion", "", "none", "", "200", "application/json", "inline", "sha256:238da5fa8009ced78a1114d0a447d0d949b78191769d174c663df2c928685718"},
		{"POST", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload", "uploadKnowledgeDocument", "cookie+csrf", true, "knowledge.ingestion", "multipart/form-data", "ref", "#/components/schemas/UploadKnowledgeDocumentRequest", "202", "application/json", "inline", "sha256:8b1b1c4e08ce7a1663a811aeeb7000d8022440ee73ea51888e1b1c8ff7effc03"},
		{"DELETE", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "deleteKnowledgeDocument", "cookie+csrf", true, "knowledge.ingestion", "", "none", "", "204", "", "none", ""},
		{"PUT", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "updateKnowledgeDocument", "cookie+csrf", true, "knowledge.ingestion", "application/json", "ref", "#/components/schemas/CreateDocumentRequest", "200", "application/json", "inline", "sha256:7f200d9c163803f673f4209c7d508bd0ef35e048d035cc73101a3aa0d7b7c4d1"},
		{"GET", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks", "listKnowledgeDocumentChunks", "cookie", false, "knowledge.retrieval", "", "none", "", "200", "application/json", "inline", "sha256:4406c6e89080bde51760127049ef8e2998e42079ff36475ce7ef560a2871f984"},
		{"PUT", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}", "updateKnowledgeDocumentChunk", "cookie+csrf", true, "knowledge.ingestion", "application/json", "ref", "#/components/schemas/UpdateKnowledgeDocumentChunkRequest", "200", "application/json", "inline", "sha256:5f82af51528b22f8fb841e50bebdd8f5c82d647ea3fe799a7ae7db73d379b036"},
		{"POST", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge", "mergeKnowledgeDocumentChunks", "cookie+csrf", true, "knowledge.ingestion", "application/json", "ref", "#/components/schemas/MergeKnowledgeDocumentChunksRequest", "200", "application/json", "inline", "sha256:4406c6e89080bde51760127049ef8e2998e42079ff36475ce7ef560a2871f984"},
		{"POST", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split", "splitKnowledgeDocumentChunk", "cookie+csrf", true, "knowledge.ingestion", "application/json", "ref", "#/components/schemas/SplitKnowledgeDocumentChunkRequest", "200", "application/json", "inline", "sha256:4406c6e89080bde51760127049ef8e2998e42079ff36475ce7ef560a2871f984"},
		{"GET", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions", "listKnowledgeDocumentVersions", "cookie", false, "knowledge.retrieval", "", "none", "", "200", "application/json", "inline", "sha256:518425b0c36324ed6d48dac2daea83c4bea309403ae5808a582fd21dd4780716"},
		{"GET", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "listKnowledgeRetrievalTestCases", "cookie", false, "knowledge.retrieval", "", "none", "", "200", "application/json", "inline", "sha256:d1701e1404f2c54605a11e88f0c332c31599ea210d02a49a9608477537764732"},
		{"POST", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "createKnowledgeRetrievalTestCase", "cookie+csrf", true, "knowledge.retrieval", "application/json", "ref", "#/components/schemas/CreateKnowledgeRetrievalTestCaseRequest", "201", "application/json", "inline", "sha256:68e2691ee00ad534f6a256c97ad3092ae041e88c6773a97b8a3f6b5ee7b28f39"},
		{"POST", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run", "runKnowledgeRetrievalTestCases", "cookie+csrf", true, "knowledge.retrieval", "application/json", "ref", "#/components/schemas/KnowledgeRetrievalTestRunRequest", "200", "application/json", "inline", "sha256:cc1a3dd0fda631c65e15089652402a35d04498d4ebc7012cc80ca0fd414da487"},
		{"POST", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve", "retrieveKnowledge", "cookie+csrf", true, "knowledge.retrieval", "application/json", "ref", "#/components/schemas/RetrieveKnowledgeRequest", "200", "application/json", "inline", "sha256:137a6b00a4c831f902aed370f13f8d6d38668bb731f78856386df95a6a52d7ee"},
		{"POST", "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve/debug", "retrieveKnowledgeDebug", "cookie+csrf", true, "knowledge.retrieval", "application/json", "ref", "#/components/schemas/RetrieveKnowledgeRequest", "200", "application/json", "inline", "sha256:5748024d0e6723faa610e4c197824b62c12ce42a3868ac332518bdbced782569"},
	})
}

func knowledgeRouteHandler(prefix string, knowledgeHandler knowledgeHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == prefix {
			if r.Method == stdhttp.MethodGet {
				knowledgeHandler.listKnowledgeBases(w, r)
			} else {
				knowledgeHandler.createKnowledgeBase(w, r)
			}
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix+"/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		knowledgeBaseID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				knowledgeHandler.getKnowledgeBase(w, r, knowledgeBaseID)
			case stdhttp.MethodPut:
				knowledgeHandler.updateKnowledgeBase(w, r, knowledgeBaseID)
			default:
				knowledgeHandler.deleteKnowledgeBase(w, r, knowledgeBaseID)
			}
			return
		}
		if len(parts) == 2 && parts[1] == "documents" {
			if r.Method == stdhttp.MethodGet {
				knowledgeHandler.listKnowledgeDocuments(w, r, knowledgeBaseID)
			} else {
				knowledgeHandler.createKnowledgeDocument(w, r, knowledgeBaseID)
			}
			return
		}
		if len(parts) == 3 && parts[1] == "documents" && parts[2] == "upload" {
			knowledgeHandler.uploadKnowledgeDocument(w, r, knowledgeBaseID)
			return
		}
		if len(parts) == 3 && parts[1] == "documents" && parts[2] == "ingestion-jobs" {
			knowledgeHandler.listKnowledgeDocumentIngestionJobs(w, r, knowledgeBaseID)
			return
		}
		if len(parts) == 2 && parts[1] == "retrieve" {
			knowledgeHandler.retrieveKnowledge(w, r, knowledgeBaseID)
			return
		}
		if len(parts) == 3 && parts[1] == "retrieve" && parts[2] == "debug" {
			knowledgeHandler.retrieveKnowledgeDebug(w, r, knowledgeBaseID)
			return
		}
		if len(parts) == 2 && parts[1] == "retrieval-test-cases" {
			if r.Method == stdhttp.MethodGet {
				knowledgeHandler.listRetrievalTestCases(w, r, knowledgeBaseID)
			} else {
				knowledgeHandler.createRetrievalTestCase(w, r, knowledgeBaseID)
			}
			return
		}
		if len(parts) == 3 && parts[1] == "retrieval-test-cases" && parts[2] == "run" {
			knowledgeHandler.runRetrievalTestCases(w, r, knowledgeBaseID)
			return
		}
		if len(parts) == 4 && parts[1] == "documents" && parts[2] != "" && parts[3] == "versions" {
			knowledgeHandler.listKnowledgeDocumentVersions(w, r, knowledgeBaseID, parts[2])
			return
		}
		if len(parts) == 3 && parts[1] == "documents" && parts[2] != "" {
			if r.Method == stdhttp.MethodPut {
				knowledgeHandler.updateKnowledgeDocument(w, r, knowledgeBaseID, parts[2])
			} else {
				knowledgeHandler.deleteKnowledgeDocument(w, r, knowledgeBaseID, parts[2])
			}
			return
		}
		if len(parts) == 4 && parts[1] == "documents" && parts[2] != "" && parts[3] == "chunks" {
			knowledgeHandler.listKnowledgeDocumentChunks(w, r, knowledgeBaseID, parts[2])
			return
		}
		if len(parts) == 5 && parts[1] == "documents" && parts[2] != "" && parts[3] == "chunks" && parts[4] != "" {
			knowledgeHandler.updateKnowledgeDocumentChunk(w, r, knowledgeBaseID, parts[2], parts[4])
			return
		}
		if len(parts) == 6 && parts[1] == "documents" && parts[2] != "" && parts[3] == "chunks" && parts[4] != "" {
			switch parts[5] {
			case "split":
				knowledgeHandler.splitKnowledgeDocumentChunk(w, r, knowledgeBaseID, parts[2], parts[4])
			case "merge":
				knowledgeHandler.mergeKnowledgeDocumentChunks(w, r, knowledgeBaseID, parts[2], parts[4])
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})
}

func registerKnowledgeRouteSurfaces(registrar *RouteSurfaceRegistrar, knowledgeHandler knowledgeHandler) error {
	operations := knowledgeRouteSurfaceOperations()
	return registerRouteSurfaceBindings(registrar, routeSurfaceBindingsForHandler(operations, RouteSurfaceAuthSession, knowledgeRouteHandler("/api/v1/app/knowledge-bases", knowledgeHandler)))
}

func registerKnowledgeRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, knowledgeHandler knowledgeHandler) {
	if err := registerKnowledgeRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), knowledgeHandler); err != nil {
		panic(err)
	}
}

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/knowledge"
	knowledgedocument "oblivious/server/internal/knowledge/document"
)

type knowledgeHandler struct {
	service *knowledge.Service
}

const knowledgeDocumentUploadMaxBytes = 10 * 1024 * 1024

type createKnowledgeBaseRequest struct {
	ChunkOverlap   int     `json:"chunkOverlap"`
	ChunkSize      int     `json:"chunkSize"`
	ChunkStrategy  string  `json:"chunkStrategy"`
	EmbeddingModel string  `json:"embeddingModel"`
	KeywordWeight  float64 `json:"keywordWeight"`
	MinScore       float64 `json:"minScore"`
	Name           string  `json:"name"`
	RerankTopK     int     `json:"rerankTopK"`
	RerankerModel  string  `json:"rerankerModel"`
	RetrievalLimit int     `json:"retrievalLimit"`
	RetrievalMode  string  `json:"retrievalMode"`
	UpdateStrategy string  `json:"updateStrategy"`
	VectorWeight   float64 `json:"vectorWeight"`
}

type createKnowledgeDocumentRequest struct {
	Content         string `json:"content"`
	DocumentVersion string `json:"documentVersion"`
	PageNumber      int    `json:"pageNumber"`
	SourceURL       string `json:"sourceUrl"`
	Title           string `json:"title"`
	UpdateStrategy  string `json:"updateStrategy"`
}

type retrieveKnowledgeRequest struct {
	AllVersions     bool     `json:"allVersions"`
	BenchmarkModes  []string `json:"benchmarkModes"`
	DocumentVersion string   `json:"documentVersion"`
	KeywordWeight   float64  `json:"keywordWeight"`
	Limit           int      `json:"limit"`
	MinScore        float64  `json:"minScore"`
	Mode            string   `json:"mode"`
	Query           string   `json:"query"`
	VectorWeight    float64  `json:"vectorWeight"`
}

type createKnowledgeRetrievalTestCaseRequest struct {
	ExpectedResult knowledge.KnowledgeRetrievalResult `json:"expectedResult"`
	Query          string                             `json:"query"`
}

type updateKnowledgeDocumentChunkRequest struct {
	Content string `json:"content"`
}

type splitKnowledgeDocumentChunkRequest struct {
	SplitAt int `json:"splitAt"`
}

type mergeKnowledgeDocumentChunksRequest struct {
	Direction string `json:"direction"`
}

type knowledgeIngestionJobResponse struct {
	ID              string     `json:"id"`
	KnowledgeBaseID string     `json:"knowledgeBaseId"`
	DocumentID      string     `json:"documentId,omitempty"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	Error           string     `json:"error,omitempty"`
	Attempts        int        `json:"attempts"`
	MaxAttempts     int        `json:"maxAttempts"`
	AvailableAt     time.Time  `json:"availableAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func knowledgeIngestionJobAPIResponse(job knowledge.KnowledgeIngestionJob) knowledgeIngestionJobResponse {
	return knowledgeIngestionJobResponse{
		ID:              job.ID,
		KnowledgeBaseID: job.KnowledgeBaseID,
		DocumentID:      job.DocumentID,
		Title:           job.Title,
		Status:          job.Status,
		Error:           job.Error,
		Attempts:        job.Attempts,
		MaxAttempts:     job.MaxAttempts,
		AvailableAt:     job.AvailableAt,
		CompletedAt:     job.CompletedAt,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
	}
}

func knowledgeIngestionJobsAPIResponse(jobs []knowledge.KnowledgeIngestionJob) []knowledgeIngestionJobResponse {
	response := make([]knowledgeIngestionJobResponse, 0, len(jobs))
	for _, job := range jobs {
		response = append(response, knowledgeIngestionJobAPIResponse(job))
	}
	return response
}

func newKnowledgeHandler(service *knowledge.Service) knowledgeHandler {
	return knowledgeHandler{service: service}
}

func (h knowledgeHandler) listKnowledgeBases(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	bases, err := h.service.List(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list knowledge bases failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, bases)
}

func (h knowledgeHandler) getKnowledgeBase(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	base, err := h.service.Get(r.Context(), session, knowledgeBaseID)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get knowledge base failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, base)
}

func (h knowledgeHandler) createKnowledgeBase(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createKnowledgeBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	base, err := h.service.CreateWithConfig(r.Context(), session, name, knowledgeBaseConfigFromRequest(payload))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create knowledge base failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, base)
}

func (h knowledgeHandler) updateKnowledgeBase(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createKnowledgeBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	base, err := h.service.UpdateWithConfig(r.Context(), session, knowledgeBaseID, name, knowledgeBaseConfigFromRequest(payload))
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update knowledge base failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, base)
}

func (h knowledgeHandler) deleteKnowledgeBase(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.service.Delete(r.Context(), session, knowledgeBaseID); err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "delete knowledge base failed")
		return
	}

	w.WriteHeader(stdhttp.StatusNoContent)
}

func (h knowledgeHandler) listKnowledgeDocuments(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	documents, err := h.service.ListDocuments(r.Context(), session, knowledgeBaseID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list knowledge documents failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, documents)
}

func (h knowledgeHandler) listKnowledgeDocumentChunks(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	chunks, err := h.service.ListDocumentChunks(r.Context(), session, knowledgeBaseID, documentID)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge document not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list knowledge document chunks failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, chunks)
}

func (h knowledgeHandler) listKnowledgeDocumentVersions(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	versions, err := h.service.ListDocumentVersions(r.Context(), session, knowledgeBaseID, documentID)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge document not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list knowledge document versions failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, versions)
}

func (h knowledgeHandler) updateKnowledgeDocumentChunk(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID, chunkID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload updateKnowledgeDocumentChunkRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	chunk, err := h.service.UpdateDocumentChunk(r.Context(), session, knowledgeBaseID, documentID, chunkID, payload.Content)
	if err != nil {
		if errors.Is(err, knowledge.ErrEmptyKnowledgeDocumentChunk) {
			writeError(w, stdhttp.StatusBadRequest, "empty_chunk_content", err.Error())
			return
		}
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge document chunk not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update knowledge document chunk failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, chunk)
}

func (h knowledgeHandler) splitKnowledgeDocumentChunk(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID, chunkID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload splitKnowledgeDocumentChunkRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	chunks, err := h.service.SplitDocumentChunk(r.Context(), session, knowledgeBaseID, documentID, chunkID, payload.SplitAt)
	if err != nil {
		if errors.Is(err, knowledge.ErrInvalidKnowledgeDocumentChunkEdit) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_chunk_edit", err.Error())
			return
		}
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge document chunk not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "split knowledge document chunk failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, chunks)
}

func (h knowledgeHandler) mergeKnowledgeDocumentChunks(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID, chunkID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload mergeKnowledgeDocumentChunksRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	chunks, err := h.service.MergeDocumentChunks(r.Context(), session, knowledgeBaseID, documentID, chunkID, payload.Direction)
	if err != nil {
		if errors.Is(err, knowledge.ErrInvalidKnowledgeDocumentChunkEdit) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_chunk_edit", err.Error())
			return
		}
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge document chunk not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "merge knowledge document chunks failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, chunks)
}

func (h knowledgeHandler) createKnowledgeDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createKnowledgeDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	title := strings.TrimSpace(payload.Title)
	content := strings.TrimSpace(payload.Content)
	if title == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "title is required")
		return
	}

	document, err := h.service.CreateDocumentWithOptions(r.Context(), session, knowledgeBaseID, title, content, knowledge.KnowledgeDocumentOptions{
		DocumentVersion: strings.TrimSpace(payload.DocumentVersion),
		PageNumber:      payload.PageNumber,
		SourceURL:       strings.TrimSpace(payload.SourceURL),
		UpdateStrategy:  strings.TrimSpace(payload.UpdateStrategy),
	})
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create knowledge document failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, document)
}

func (h knowledgeHandler) uploadKnowledgeDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := r.ParseMultipartForm(knowledgeDocumentUploadMaxBytes); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "file is required")
		return
	}
	defer file.Close()

	contentType := ""
	filename := ""
	if header != nil {
		contentType = header.Header.Get("Content-Type")
		filename = header.Filename
	}
	rawContent, err := readKnowledgeUploadRawFile(file)
	if err != nil {
		writeKnowledgeUploadError(w, err)
		return
	}
	parsed, err := knowledgedocument.NewParser().Parse(r.Context(), bytes.NewReader(rawContent), filename, contentType, knowledgeDocumentUploadMaxBytes)
	if err != nil {
		writeKnowledgeUploadError(w, err)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = parsed.Title
	}
	if title == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "title is required")
		return
	}

	job, err := h.service.EnqueueDocumentIngestion(
		r.Context(),
		session,
		knowledgeBaseID,
		title,
		parsed.Content,
		knowledge.KnowledgeDocumentOptions{
			DocumentVersion: strings.TrimSpace(r.FormValue("documentVersion")),
			PageNumber:      parseKnowledgeDocumentPageNumber(r.FormValue("pageNumber")),
			SourceURL:       strings.TrimSpace(r.FormValue("sourceUrl")),
			UpdateStrategy:  strings.TrimSpace(r.FormValue("updateStrategy")),
		},
		knowledge.KnowledgeIngestionRawPayload{
			Content:     rawContent,
			Filename:    filename,
			ContentType: contentType,
		},
	)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "upload knowledge document failed")
		return
	}

	writeSuccess(w, stdhttp.StatusAccepted, knowledgeIngestionJobAPIResponse(job))
}

func readKnowledgeUploadRawFile(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, knowledgedocument.ErrEmptyDocument
	}
	rawContent, err := io.ReadAll(io.LimitReader(reader, knowledgeDocumentUploadMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(rawContent)) > knowledgeDocumentUploadMaxBytes {
		return nil, knowledgedocument.ErrDocumentTooLarge
	}
	return rawContent, nil
}

func parseKnowledgeDocumentPageNumber(raw string) int {
	pageNumber, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || pageNumber < 0 {
		return 0
	}
	return pageNumber
}

func (h knowledgeHandler) listKnowledgeDocumentIngestionJobs(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	jobs, err := h.service.ListDocumentIngestionJobs(r.Context(), session, knowledgeBaseID)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list knowledge document ingestion jobs failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, knowledgeIngestionJobsAPIResponse(jobs))
}

func (h knowledgeHandler) retrieveKnowledge(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload retrieveKnowledgeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	query := strings.TrimSpace(payload.Query)
	if query == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "query is required")
		return
	}

	rerankTopK, err := h.knowledgeBaseRerankTopK(r.Context(), session, knowledgeBaseID, payload.Mode)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "load knowledge base config failed")
		return
	}

	results, err := h.service.RetrieveWithOptions(r.Context(), session, knowledgeBaseID, query, knowledge.KnowledgeRetrievalOptions{
		AllVersions:     payload.AllVersions,
		DocumentVersion: strings.TrimSpace(payload.DocumentVersion),
		Mode:            strings.TrimSpace(payload.Mode),
		Limit:           payload.Limit,
		MinScore:        payload.MinScore,
		RerankTopK:      rerankTopK,
		VectorWeight:    payload.VectorWeight,
		KeywordWeight:   payload.KeywordWeight,
	})
	if err != nil {
		if errors.Is(err, knowledge.ErrInvalidKnowledgeRetrievalOptions) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_retrieval_options", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "retrieve knowledge failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, results)
}

func (h knowledgeHandler) retrieveKnowledgeDebug(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload retrieveKnowledgeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	query := strings.TrimSpace(payload.Query)
	if query == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "query is required")
		return
	}

	rerankTopK, err := h.knowledgeBaseRerankTopK(r.Context(), session, knowledgeBaseID, payload.Mode)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "load knowledge base config failed")
		return
	}

	options := knowledge.KnowledgeRetrievalOptions{
		AllVersions:     payload.AllVersions,
		DocumentVersion: strings.TrimSpace(payload.DocumentVersion),
		Mode:            strings.TrimSpace(payload.Mode),
		Limit:           payload.Limit,
		MinScore:        payload.MinScore,
		RerankTopK:      rerankTopK,
		VectorWeight:    payload.VectorWeight,
		KeywordWeight:   payload.KeywordWeight,
	}
	results, err := h.service.RetrieveWithOptions(r.Context(), session, knowledgeBaseID, query, options)
	if err != nil {
		if errors.Is(err, knowledge.ErrInvalidKnowledgeRetrievalOptions) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_retrieval_options", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "retrieve knowledge failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, knowledge.KnowledgeRetrievalDebugReport{
		KnowledgeBaseID:  knowledgeBaseID,
		Query:            query,
		Options:          options,
		ResultCount:      len(results),
		CitationCoverage: knowledgeRetrievalCitationCoverage(results),
		Results:          results,
	})
}

func (h knowledgeHandler) createRetrievalTestCase(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createKnowledgeRetrievalTestCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	testCase, err := h.service.CreateRetrievalTestCase(r.Context(), session, knowledgeBaseID, knowledge.CreateKnowledgeRetrievalTestCaseRequest{
		ExpectedResult: payload.ExpectedResult,
		Query:          payload.Query,
	})
	if err != nil {
		if errors.Is(err, knowledge.ErrInvalidKnowledgeRetrievalTestCase) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_retrieval_test_case", err.Error())
			return
		}
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create retrieval test case failed")
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, testCase)
}

func (h knowledgeHandler) listRetrievalTestCases(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	testCases, err := h.service.ListRetrievalTestCases(r.Context(), session, knowledgeBaseID)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list retrieval test cases failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, testCases)
}

func (h knowledgeHandler) runRetrievalTestCases(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload retrieveKnowledgeRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
			return
		}
	}

	rerankTopK, err := h.knowledgeBaseRerankTopK(r.Context(), session, knowledgeBaseID, payload.Mode)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "load knowledge base config failed")
		return
	}

	report, err := h.service.RunRetrievalTestCases(r.Context(), session, knowledgeBaseID, knowledge.KnowledgeRetrievalOptions{
		AllVersions:     payload.AllVersions,
		BenchmarkModes:  payload.BenchmarkModes,
		DocumentVersion: strings.TrimSpace(payload.DocumentVersion),
		Mode:            strings.TrimSpace(payload.Mode),
		Limit:           payload.Limit,
		MinScore:        payload.MinScore,
		RerankTopK:      rerankTopK,
		VectorWeight:    payload.VectorWeight,
		KeywordWeight:   payload.KeywordWeight,
	})
	if err != nil {
		if errors.Is(err, knowledge.ErrInvalidKnowledgeRetrievalOptions) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_retrieval_options", err.Error())
			return
		}
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge base not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "run retrieval test cases failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, report)
}

func (h knowledgeHandler) updateKnowledgeDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createKnowledgeDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	title := strings.TrimSpace(payload.Title)
	content := strings.TrimSpace(payload.Content)
	if title == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "title is required")
		return
	}

	document, err := h.service.UpdateDocumentWithOptions(r.Context(), session, knowledgeBaseID, documentID, title, content, knowledge.KnowledgeDocumentOptions{
		DocumentVersion: strings.TrimSpace(payload.DocumentVersion),
		UpdateStrategy:  strings.TrimSpace(payload.UpdateStrategy),
	})
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge document not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update knowledge document failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, document)
}

func (h knowledgeHandler) deleteKnowledgeDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.service.DeleteDocument(r.Context(), session, knowledgeBaseID, documentID); err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge document not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "delete knowledge document failed")
		return
	}

	w.WriteHeader(stdhttp.StatusNoContent)
}

func (h knowledgeHandler) deleteKnowledgeDocumentByID(w stdhttp.ResponseWriter, r *stdhttp.Request, documentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.service.DeleteDocumentByID(r.Context(), session, documentID); err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "knowledge document not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "delete knowledge document failed")
		return
	}

	w.WriteHeader(stdhttp.StatusNoContent)
}

func knowledgeBaseConfigFromRequest(payload createKnowledgeBaseRequest) knowledge.KnowledgeBaseConfig {
	return knowledge.KnowledgeBaseConfig{
		ChunkOverlap:   payload.ChunkOverlap,
		ChunkSize:      payload.ChunkSize,
		ChunkStrategy:  strings.TrimSpace(payload.ChunkStrategy),
		EmbeddingModel: strings.TrimSpace(payload.EmbeddingModel),
		KeywordWeight:  payload.KeywordWeight,
		MinScore:       payload.MinScore,
		RerankTopK:     payload.RerankTopK,
		RerankerModel:  strings.TrimSpace(payload.RerankerModel),
		RetrievalLimit: payload.RetrievalLimit,
		RetrievalMode:  strings.TrimSpace(payload.RetrievalMode),
		UpdateStrategy: strings.TrimSpace(payload.UpdateStrategy),
		VectorWeight:   payload.VectorWeight,
	}
}

func (h knowledgeHandler) knowledgeBaseRerankTopK(ctx context.Context, session auth.Session, knowledgeBaseID, mode string) (int, error) {
	if strings.TrimSpace(mode) != knowledge.KnowledgeRetrievalModeHybridRerank {
		return 0, nil
	}
	base, err := h.service.Get(ctx, session, knowledgeBaseID)
	if err != nil {
		return 0, err
	}
	return base.RerankTopK, nil
}

func knowledgeRetrievalCitationCoverage(results []knowledge.KnowledgeRetrievalResult) knowledge.KnowledgeRetrievalCitationCoverage {
	coverage := knowledge.KnowledgeRetrievalCitationCoverage{TotalResults: len(results)}
	for _, result := range results {
		source := result.Source
		if strings.TrimSpace(source.SourceURL) != "" ||
			strings.TrimSpace(source.DocumentID) != "" ||
			strings.TrimSpace(source.ChunkID) != "" {
			coverage.ResultsWithSource++
		}
		if source.PageNumber > 0 {
			coverage.ResultsWithPage++
		}
		if len(source.HighlightPositions) > 0 {
			coverage.ResultsWithHighlights++
		}
		if strings.TrimSpace(source.OriginalText) != "" {
			coverage.ResultsWithOriginalText++
		}
	}
	return coverage
}

func writeKnowledgeUploadError(w stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, knowledge.ErrUnsupportedKnowledgeDocumentFormat), errors.Is(err, knowledgedocument.ErrUnsupportedDocumentFormat):
		writeError(w, stdhttp.StatusBadRequest, "unsupported_document_format", err.Error())
	case errors.Is(err, knowledge.ErrKnowledgeDocumentTooLarge), errors.Is(err, knowledgedocument.ErrDocumentTooLarge):
		writeError(w, stdhttp.StatusRequestEntityTooLarge, "document_too_large", err.Error())
	case errors.Is(err, knowledge.ErrEmptyKnowledgeDocument), errors.Is(err, knowledgedocument.ErrEmptyDocument):
		writeError(w, stdhttp.StatusBadRequest, "empty_document", err.Error())
	case errors.Is(err, io.ErrUnexpectedEOF):
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid uploaded file")
	default:
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "parse uploaded document failed")
	}
}

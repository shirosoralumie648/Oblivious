package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/relay/types"
)

// Document 内存文档
type Document struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	UserID         string         `json:"userId"`
	Title          string         `json:"title,omitempty"`
	Content        string         `json:"content"`
	SourceType     string         `json:"sourceType"`
	SourceURL      string         `json:"sourceUrl,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	TotalChunks    int            `json:"totalChunks"`
	EmbeddingModel string         `json:"embeddingModel"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// Chunk 文档分块
type Chunk struct {
	ID             string         `json:"id"`
	DocumentID     string         `json:"documentId"`
	OrganizationID string         `json:"organizationId"`
	UserID         string         `json:"userId"`
	Content        string         `json:"content"`
	ChunkIndex     int            `json:"chunkIndex"`
	Embedding      []float32      `json:"embedding,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

// SearchResult 搜索结果
type SearchResult struct {
	DocumentID    string  `json:"documentId"`
	DocumentTitle string  `json:"documentTitle"`
	ChunkContent  string  `json:"chunkContent"`
	ChunkIndex    int     `json:"chunkIndex"`
	Score         float64 `json:"score"` // 相似度分数 (0-1, 越高越相似)
}

// AddDocumentRequest 添加文档请求
type AddDocumentRequest struct {
	Title      string         `json:"title,omitempty"`
	Content    string         `json:"content"`
	SourceType string         `json:"sourceType,omitempty"`
	SourceURL  string         `json:"sourceUrl,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query    string  `json:"query"`
	TopK     int     `json:"topK,omitempty"`
	MinScore float64 `json:"minScore,omitempty"`
}

// Store 存储接口
type Store interface {
	CreateDocument(ctx context.Context, doc *Document) (*Document, error)
	GetDocument(ctx context.Context, id, organizationID string) (*Document, error)
	ListDocuments(ctx context.Context, userID, organizationID string, limit, offset int) ([]*Document, error)
	UpdateDocument(ctx context.Context, id, organizationID string, title, content string) (*Document, error)
	DeleteDocument(ctx context.Context, id, organizationID string) error

	CreateChunk(ctx context.Context, chunk *Chunk) (*Chunk, error)
	CreateChunks(ctx context.Context, chunks []*Chunk) error
	ListChunks(ctx context.Context, documentID, organizationID string) ([]*Chunk, error)
	DeleteChunks(ctx context.Context, documentID, organizationID string) error

	SearchSimilar(ctx context.Context, userID, organizationID string, embedding []float32, topK int, minScore float64) ([]*SearchResult, error)
}

// Embedder 嵌入接口
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Chunker 分块接口
type Chunker interface {
	Chunk(text string) []string
}

// Service Memory 服务
type Service struct {
	store    Store
	embedder Embedder
	chunker  Chunker
	model    string // 默认嵌入模型
}

// NewService 创建 Service
func NewService(store Store, embedder Embedder, chunker Chunker, model string) *Service {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &Service{
		store:    store,
		embedder: embedder,
		chunker:  chunker,
		model:    model,
	}
}

// AddDocument 添加文档
func (s *Service) AddDocument(ctx context.Context, session auth.Session, req *AddDocumentRequest) (*Document, error) {
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	now := time.Now().UTC()
	docID, err := auth.NewID("memdoc")
	if err != nil {
		return nil, err
	}

	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = "manual"
	}

	doc := &Document{
		ID:             docID,
		OrganizationID: session.OrganizationID,
		UserID:         session.User.ID,
		Title:          req.Title,
		Content:        req.Content,
		SourceType:     sourceType,
		SourceURL:      req.SourceURL,
		Metadata:       req.Metadata,
		TotalChunks:    0,
		EmbeddingModel: s.model,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 创建文档记录
	created, err := s.store.CreateDocument(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}

	// 分块
	chunks := s.chunker.Chunk(req.Content)
	if len(chunks) == 0 {
		return created, nil
	}

	// 批量嵌入
	ctx = withSessionRelayIdentity(ctx, session)
	embeddings, err := s.embedder.EmbedBatch(ctx, chunks)
	if err != nil {
		// 嵌入失败，删除文档
		s.store.DeleteDocument(ctx, docID, session.OrganizationID)
		return nil, fmt.Errorf("embed chunks: %w", err)
	}

	// 创建 chunk 记录
	chunkRecords := make([]*Chunk, len(chunks))
	for i, content := range chunks {
		chunkID, err := auth.NewID("memchunk")
		if err != nil {
			return nil, err
		}

		var embedding []float32
		if i < len(embeddings) {
			embedding = embeddings[i]
		}

		chunkRecords[i] = &Chunk{
			ID:             chunkID,
			DocumentID:     docID,
			OrganizationID: session.OrganizationID,
			UserID:         session.User.ID,
			Content:        content,
			ChunkIndex:     i,
			Embedding:      embedding,
			Metadata:       map[string]any{},
			CreatedAt:      now,
		}
	}

	if err := s.store.CreateChunks(ctx, chunkRecords); err != nil {
		s.store.DeleteDocument(ctx, docID, session.OrganizationID)
		return nil, fmt.Errorf("save chunks: %w", err)
	}

	created.TotalChunks = len(chunks)
	return created, nil
}

// GetDocument 获取文档
func (s *Service) GetDocument(ctx context.Context, session auth.Session, id string) (*Document, error) {
	doc, err := s.store.GetDocument(ctx, id, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}
	if doc.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}
	return doc, nil
}

// ListDocuments 列出用户的文档
func (s *Service) ListDocuments(ctx context.Context, session auth.Session, limit, offset int) ([]*Document, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.ListDocuments(ctx, session.User.ID, session.OrganizationID, limit, offset)
}

// UpdateDocument 更新文档
func (s *Service) UpdateDocument(ctx context.Context, session auth.Session, id string, title, content string) (*Document, error) {
	// 验证所有权
	doc, err := s.store.GetDocument(ctx, id, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}
	if doc.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	// 更新文档
	updated, err := s.store.UpdateDocument(ctx, id, session.OrganizationID, title, content)
	if err != nil {
		return nil, err
	}

	// 如果内容变化，重新分块和嵌入
	if content != doc.Content {
		// 删除旧 chunks
		if err := s.store.DeleteChunks(ctx, id, session.OrganizationID); err != nil {
			return nil, fmt.Errorf("delete old chunks: %w", err)
		}

		// 分块
		chunks := s.chunker.Chunk(content)
		if len(chunks) == 0 {
			return updated, nil
		}

		// 嵌入
		ctx = withSessionRelayIdentity(ctx, session)
		embeddings, err := s.embedder.EmbedBatch(ctx, chunks)
		if err != nil {
			return nil, fmt.Errorf("embed chunks: %w", err)
		}

		// 创建新 chunks
		now := time.Now().UTC()
		chunkRecords := make([]*Chunk, len(chunks))
		for i, chunkContent := range chunks {
			chunkID, err := auth.NewID("memchunk")
			if err != nil {
				return nil, err
			}

			var embedding []float32
			if i < len(embeddings) {
				embedding = embeddings[i]
			}

			chunkRecords[i] = &Chunk{
				ID:             chunkID,
				DocumentID:     id,
				OrganizationID: session.OrganizationID,
				UserID:         session.User.ID,
				Content:        chunkContent,
				ChunkIndex:     i,
				Embedding:      embedding,
				Metadata:       map[string]any{},
				CreatedAt:      now,
			}
		}

		if err := s.store.CreateChunks(ctx, chunkRecords); err != nil {
			return nil, fmt.Errorf("save chunks: %w", err)
		}

		updated.TotalChunks = len(chunks)
	}

	return updated, nil
}

// DeleteDocument 删除文档
func (s *Service) DeleteDocument(ctx context.Context, session auth.Session, id string) error {
	// 验证所有权
	doc, err := s.store.GetDocument(ctx, id, session.OrganizationID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}
	if doc.UserID != session.User.ID {
		return fmt.Errorf("access denied")
	}

	return s.store.DeleteDocument(ctx, id, session.OrganizationID)
}

// Search 搜索相似内容
func (s *Service) Search(ctx context.Context, session auth.Session, req *SearchRequest) ([]*SearchResult, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}

	minScore := req.MinScore
	if minScore <= 0 {
		minScore = 0.5
	}

	// 嵌入查询
	ctx = withSessionRelayIdentity(ctx, session)
	queryEmbedding, err := s.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// 向量搜索
	return s.store.SearchSimilar(ctx, session.User.ID, session.OrganizationID, queryEmbedding, topK, minScore)
}

// ListChunks 列出文档的分块
func (s *Service) ListChunks(ctx context.Context, session auth.Session, documentID string) ([]*Chunk, error) {
	// 验证所有权
	doc, err := s.store.GetDocument(ctx, documentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}
	if doc.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	return s.store.ListChunks(ctx, documentID, session.OrganizationID)
}

func withSessionRelayIdentity(ctx context.Context, session auth.Session) context.Context {
	if session.User.ID != "" {
		ctx = types.WithTrustedUserID(ctx, session.User.ID)
	}
	if session.OrganizationID != "" {
		ctx = types.WithTrustedOrganizationID(ctx, session.OrganizationID)
	}
	return ctx
}

// SQLStore SQL 实现
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore 创建 SQLStore
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// CreateDocument 创建文档
func (s *SQLStore) CreateDocument(ctx context.Context, doc *Document) (*Document, error) {
	metadataJSON, _ := json.Marshal(doc.Metadata)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_documents (id, user_id, organization_id, title, content, source_type, source_url, metadata, total_chunks, embedding_model, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, doc.ID, doc.UserID, doc.OrganizationID, doc.Title, doc.Content, doc.SourceType, doc.SourceURL, metadataJSON, doc.TotalChunks, doc.EmbeddingModel, doc.CreatedAt, doc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert document: %w", err)
	}

	return doc, nil
}

// GetDocument 获取文档
func (s *SQLStore) GetDocument(ctx context.Context, id, organizationID string) (*Document, error) {
	var doc Document
	var metadataJSON []byte
	var title, sourceURL sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, user_id, title, content, source_type, source_url, metadata, total_chunks, embedding_model, created_at, updated_at
		FROM memory_documents WHERE id = $1 AND organization_id = $2
	`, id, organizationID).Scan(&doc.ID, &doc.OrganizationID, &doc.UserID, &title, &doc.Content, &doc.SourceType, &sourceURL, &metadataJSON, &doc.TotalChunks, &doc.EmbeddingModel, &doc.CreatedAt, &doc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	doc.Title = title.String
	doc.SourceURL = sourceURL.String
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &doc.Metadata)
	}

	return &doc, nil
}

// ListDocuments 列出文档
func (s *SQLStore) ListDocuments(ctx context.Context, userID, organizationID string, limit, offset int) ([]*Document, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, user_id, title, content, source_type, source_url, metadata, total_chunks, embedding_model, created_at, updated_at
		FROM memory_documents WHERE user_id = $1 AND organization_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, userID, organizationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var docs []*Document
	for rows.Next() {
		var doc Document
		var metadataJSON []byte
		var title, sourceURL sql.NullString

		if err := rows.Scan(&doc.ID, &doc.OrganizationID, &doc.UserID, &title, &doc.Content, &doc.SourceType, &sourceURL, &metadataJSON, &doc.TotalChunks, &doc.EmbeddingModel, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}

		doc.Title = title.String
		doc.SourceURL = sourceURL.String
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &doc.Metadata)
		}
		docs = append(docs, &doc)
	}

	return docs, rows.Err()
}

// UpdateDocument 更新文档
func (s *SQLStore) UpdateDocument(ctx context.Context, id, organizationID string, title, content string) (*Document, error) {
	now := time.Now().UTC()

	var doc Document
	var metadataJSON []byte
	var nullTitle, sourceURL sql.NullString

	err := s.db.QueryRowContext(ctx, `
		UPDATE memory_documents
		SET title = $2, content = $3, updated_at = $4
		WHERE id = $1 AND organization_id = $5
		RETURNING id, organization_id, user_id, title, content, source_type, source_url, metadata, total_chunks, embedding_model, created_at, updated_at
	`, id, title, content, now, organizationID).Scan(&doc.ID, &doc.OrganizationID, &doc.UserID, &nullTitle, &doc.Content, &doc.SourceType, &sourceURL, &metadataJSON, &doc.TotalChunks, &doc.EmbeddingModel, &doc.CreatedAt, &doc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("update document: %w", err)
	}

	doc.Title = nullTitle.String
	doc.SourceURL = sourceURL.String
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &doc.Metadata)
	}

	return &doc, nil
}

// DeleteDocument 删除文档
func (s *SQLStore) DeleteDocument(ctx context.Context, id, organizationID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM memory_documents WHERE id = $1 AND organization_id = $2`, id, organizationID)
	return err
}

// CreateChunk 创建分块
func (s *SQLStore) CreateChunk(ctx context.Context, chunk *Chunk) (*Chunk, error) {
	metadataJSON, _ := json.Marshal(chunk.Metadata)

	// 将 embedding 转换为 PostgreSQL vector 格式
	embeddingStr := embeddingToVector(chunk.Embedding)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_chunks (id, document_id, user_id, organization_id, content, chunk_index, embedding, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, chunk.ID, chunk.DocumentID, chunk.UserID, chunk.OrganizationID, chunk.Content, chunk.ChunkIndex, embeddingStr, metadataJSON, chunk.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert chunk: %w", err)
	}

	return chunk, nil
}

// CreateChunks 批量创建分块
func (s *SQLStore) CreateChunks(ctx context.Context, chunks []*Chunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, chunk := range chunks {
		metadataJSON, _ := json.Marshal(chunk.Metadata)
		embeddingStr := embeddingToVector(chunk.Embedding)

		_, err := tx.ExecContext(ctx, `
			INSERT INTO memory_chunks (id, document_id, user_id, organization_id, content, chunk_index, embedding, metadata, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, chunk.ID, chunk.DocumentID, chunk.UserID, chunk.OrganizationID, chunk.Content, chunk.ChunkIndex, embeddingStr, metadataJSON, chunk.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert chunk %d: %w", chunk.ChunkIndex, err)
		}
	}

	// 更新文档的 total_chunks
	if len(chunks) > 0 {
		_, err := tx.ExecContext(ctx, `
			UPDATE memory_documents SET total_chunks = $2, updated_at = $3 WHERE id = $1 AND organization_id = $4
		`, chunks[0].DocumentID, len(chunks), time.Now().UTC(), chunks[0].OrganizationID)
		if err != nil {
			return fmt.Errorf("update document chunks count: %w", err)
		}
	}

	return tx.Commit()
}

// ListChunks 列出分块
func (s *SQLStore) ListChunks(ctx context.Context, documentID, organizationID string) ([]*Chunk, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, document_id, organization_id, user_id, content, chunk_index, embedding, metadata, created_at
		FROM memory_chunks WHERE document_id = $1 AND organization_id = $2
		ORDER BY chunk_index ASC
	`, documentID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		var chunk Chunk
		var metadataJSON []byte
		var embeddingStr sql.NullString

		if err := rows.Scan(&chunk.ID, &chunk.DocumentID, &chunk.OrganizationID, &chunk.UserID, &chunk.Content, &chunk.ChunkIndex, &embeddingStr, &metadataJSON, &chunk.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}

		if embeddingStr.Valid {
			chunk.Embedding = vectorToEmbedding(embeddingStr.String)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &chunk.Metadata)
		}
		chunks = append(chunks, &chunk)
	}

	return chunks, rows.Err()
}

// DeleteChunks 删除分块
func (s *SQLStore) DeleteChunks(ctx context.Context, documentID, organizationID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM memory_chunks WHERE document_id = $1 AND organization_id = $2`, documentID, organizationID)
	return err
}

// SearchSimilar 向量相似度搜索
func (s *SQLStore) SearchSimilar(ctx context.Context, userID, organizationID string, embedding []float32, topK int, minScore float64) ([]*SearchResult, error) {
	embeddingStr := embeddingToVector(embedding)

	// 使用余弦距离搜索
	// pgvector: <=> 是余弦距离操作符
	// 相似度 = 1 - 距离
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			mc.document_id,
			COALESCE(md.title, ''),
			mc.content,
			mc.chunk_index,
			1 - (mc.embedding <=> $3::vector) AS similarity
		FROM memory_chunks mc
		JOIN memory_documents md ON md.id = mc.document_id
		WHERE mc.user_id = $1 AND mc.organization_id = $2
		ORDER BY mc.embedding <=> $3::vector
		LIMIT $4
	`, userID, organizationID, embeddingStr, topK)
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		var result SearchResult
		var similarity float64

		if err := rows.Scan(&result.DocumentID, &result.DocumentTitle, &result.ChunkContent, &result.ChunkIndex, &similarity); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}

		result.Score = similarity
		if similarity >= minScore {
			results = append(results, &result)
		}
	}

	return results, rows.Err()
}

// embeddingToVector 将 embedding 转换为 PostgreSQL vector 字符串格式
func embeddingToVector(embedding []float32) string {
	if len(embedding) == 0 {
		return "[]"
	}

	result := make([]byte, 0, len(embedding)*12)
	result = append(result, '[')
	for i, v := range embedding {
		if i > 0 {
			result = append(result, ',')
		}
		result = append(result, fmt.Sprintf("%f", v)...)
	}
	result = append(result, ']')
	return string(result)
}

// vectorToEmbedding 将 PostgreSQL vector 字符串转换为 embedding
func vectorToEmbedding(vec string) []float32 {
	if vec == "" || vec == "[]" {
		return nil
	}

	// 移除 [ ]
	inner := vec[1 : len(vec)-1]
	if inner == "" {
		return nil
	}

	// 简单分割解析
	var result []float32
	start := 0
	for i := 0; i < len(inner); i++ {
		if inner[i] == ',' {
			var v float32
			fmt.Sscanf(inner[start:i], "%f", &v)
			result = append(result, v)
			start = i + 1
		}
	}
	// 最后一个元素
	if start < len(inner) {
		var v float32
		fmt.Sscanf(inner[start:], "%f", &v)
		result = append(result, v)
	}

	return result
}

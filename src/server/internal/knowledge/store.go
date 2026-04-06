package knowledge

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode"

	"oblivious/server/internal/auth"
)

const (
	knowledgeDocumentChunkSize = 280
	knowledgeRetrievalLimit    = 5
	knowledgeSnippetSize       = 220
)

func (s *SQLStore) CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (KnowledgeBase, error) {
	knowledgeBaseID, err := auth.NewID("kb")
	if err != nil {
		return KnowledgeBase{}, err
	}

	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO knowledge_bases (id, workspace_id, name, document_count, created_at, updated_at)
		VALUES ($1, $2, $3, 0, $4, $4)
	`, knowledgeBaseID, workspaceID, name, now); err != nil {
		return KnowledgeBase{}, err
	}

	return KnowledgeBase{
		DocumentCount: 0,
		ID:            knowledgeBaseID,
		Name:          name,
		UpdatedAt:     now,
	}, nil
}

func (s *SQLStore) UpdateKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID, name string) (KnowledgeBase, error) {
	var base KnowledgeBase

	if err := s.db.QueryRowContext(ctx, `
		UPDATE knowledge_bases
		SET name = $3, updated_at = $4
		WHERE workspace_id = $1 AND id = $2
		RETURNING id, name, document_count, updated_at
	`, workspaceID, knowledgeBaseID, name, time.Now().UTC()).Scan(&base.ID, &base.Name, &base.DocumentCount, &base.UpdatedAt); err != nil {
		return KnowledgeBase{}, err
	}

	return base, nil
}

func (s *SQLStore) DeleteKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM knowledge_bases
		WHERE workspace_id = $1 AND id = $2
	`, workspaceID, knowledgeBaseID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *SQLStore) ListKnowledgeBases(ctx context.Context, workspaceID string) ([]KnowledgeBase, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, document_count, updated_at
		FROM knowledge_bases
		WHERE workspace_id = $1
		ORDER BY updated_at DESC, name ASC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bases := []KnowledgeBase{}
	for rows.Next() {
		var base KnowledgeBase
		if err := rows.Scan(&base.ID, &base.Name, &base.DocumentCount, &base.UpdatedAt); err != nil {
			return nil, err
		}
		bases = append(bases, base)
	}

	return bases, rows.Err()
}

func (s *SQLStore) GetKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) (KnowledgeBase, error) {
	var base KnowledgeBase

	if err := s.db.QueryRowContext(ctx, `
		SELECT id, name, document_count, updated_at
		FROM knowledge_bases
		WHERE workspace_id = $1 AND id = $2
	`, workspaceID, knowledgeBaseID).Scan(&base.ID, &base.Name, &base.DocumentCount, &base.UpdatedAt); err != nil {
		return KnowledgeBase{}, err
	}

	return base, nil
}

func (s *SQLStore) ListKnowledgeDocuments(ctx context.Context, workspaceID, knowledgeBaseID string) ([]KnowledgeDocument, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.content, d.updated_at
		FROM knowledge_documents d
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		WHERE kb.workspace_id = $1 AND d.knowledge_base_id = $2
		ORDER BY d.updated_at DESC, d.title ASC
	`, workspaceID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	documents := []KnowledgeDocument{}
	for rows.Next() {
		var document KnowledgeDocument
		if err := rows.Scan(&document.ID, &document.Title, &document.Content, &document.UpdatedAt); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}

	return documents, rows.Err()
}

func (s *SQLStore) CreateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, title, content string) (KnowledgeDocument, error) {
	documentID, err := auth.NewID("doc")
	if err != nil {
		return KnowledgeDocument{}, err
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO knowledge_documents (id, knowledge_base_id, title, content, created_at, updated_at)
		SELECT $1, kb.id, $3, $4, $5, $5
		FROM knowledge_bases kb
		WHERE kb.workspace_id = $2 AND kb.id = $6
	`, documentID, workspaceID, title, content, now, knowledgeBaseID)
	if err != nil {
		return KnowledgeDocument{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return KnowledgeDocument{}, err
	}
	if rowsAffected == 0 {
		return KnowledgeDocument{}, sql.ErrNoRows
	}

	if err := replaceKnowledgeDocumentChunks(ctx, tx, documentID, content, now); err != nil {
		return KnowledgeDocument{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE knowledge_bases
		SET document_count = document_count + 1, updated_at = $2
		WHERE workspace_id = $1 AND id = $3
	`, workspaceID, now, knowledgeBaseID); err != nil {
		return KnowledgeDocument{}, err
	}

	if err := tx.Commit(); err != nil {
		return KnowledgeDocument{}, err
	}

	return KnowledgeDocument{
		Content:   content,
		ID:        documentID,
		Title:     title,
		UpdatedAt: now,
	}, nil
}

func (s *SQLStore) UpdateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error) {
	var document KnowledgeDocument
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	defer tx.Rollback()

	if err := tx.QueryRowContext(ctx, `
		UPDATE knowledge_documents d
		SET title = $4, content = $5, updated_at = $6
		FROM knowledge_bases kb
		WHERE d.knowledge_base_id = kb.id AND kb.workspace_id = $1 AND kb.id = $2 AND d.id = $3
		RETURNING d.id, d.title, d.content, d.updated_at
	`, workspaceID, knowledgeBaseID, documentID, title, content, now).Scan(
		&document.ID,
		&document.Title,
		&document.Content,
		&document.UpdatedAt,
	); err != nil {
		return KnowledgeDocument{}, err
	}

	if err := replaceKnowledgeDocumentChunks(ctx, tx, document.ID, content, now); err != nil {
		return KnowledgeDocument{}, err
	}

	if err := tx.Commit(); err != nil {
		return KnowledgeDocument{}, err
	}

	return document, nil
}

func (s *SQLStore) DeleteKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		DELETE FROM knowledge_documents d
		USING knowledge_bases kb
		WHERE d.knowledge_base_id = kb.id AND kb.workspace_id = $1 AND kb.id = $2 AND d.id = $3
	`, workspaceID, knowledgeBaseID, documentID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE knowledge_bases
		SET document_count = GREATEST(document_count - 1, 0), updated_at = $2
		WHERE workspace_id = $1 AND id = $3
	`, workspaceID, time.Now().UTC(), knowledgeBaseID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) RetrieveKnowledge(ctx context.Context, workspaceID, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return []KnowledgeRetrievalResult{}, nil
	}

	pattern := "%" + escapeLikePattern(trimmedQuery) + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.content, c.content
		FROM knowledge_documents d
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		LEFT JOIN knowledge_document_chunks c ON c.document_id = d.id
		WHERE kb.workspace_id = $1 AND d.knowledge_base_id = $2 AND (
			d.title ILIKE $3 ESCAPE '\'
			OR d.content ILIKE $3 ESCAPE '\'
			OR c.content ILIKE $3 ESCAPE '\'
		)
		ORDER BY
			CASE
				WHEN d.title ILIKE $3 ESCAPE '\' THEN 3
				WHEN c.content ILIKE $3 ESCAPE '\' THEN 2
				WHEN d.content ILIKE $3 ESCAPE '\' THEN 1
				ELSE 0
			END DESC,
			d.updated_at DESC,
			d.title ASC,
			COALESCE(c.chunk_index, -1) ASC
		LIMIT 20
	`, workspaceID, knowledgeBaseID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]KnowledgeRetrievalResult, 0, knowledgeRetrievalLimit)
	seen := map[string]struct{}{}
	for rows.Next() {
		var (
			documentID    string
			documentTitle string
			documentBody  string
			chunkContent  sql.NullString
		)

		if err := rows.Scan(&documentID, &documentTitle, &documentBody, &chunkContent); err != nil {
			return nil, err
		}

		source := documentBody
		if chunkContent.Valid && strings.Contains(strings.ToLower(chunkContent.String), strings.ToLower(trimmedQuery)) {
			source = chunkContent.String
		}

		snippet := buildKnowledgeSnippet(source, trimmedQuery)
		if snippet == "" {
			continue
		}

		resultKey := documentID + "|" + snippet
		if _, exists := seen[resultKey]; exists {
			continue
		}
		seen[resultKey] = struct{}{}

		results = append(results, KnowledgeRetrievalResult{
			DocumentID:    documentID,
			DocumentTitle: documentTitle,
			Snippet:       snippet,
		})
		if len(results) == knowledgeRetrievalLimit {
			break
		}
	}

	return results, rows.Err()
}

func replaceKnowledgeDocumentChunks(ctx context.Context, tx *sql.Tx, documentID, content string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM knowledge_document_chunks
		WHERE document_id = $1
	`, documentID); err != nil {
		return err
	}

	chunks := buildKnowledgeDocumentChunks(content)
	for index, chunk := range chunks {
		chunkID, err := auth.NewID("kdc")
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO knowledge_document_chunks (id, document_id, chunk_index, content, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, chunkID, documentID, index, chunk, now); err != nil {
			return err
		}
	}

	return nil
}

func buildKnowledgeDocumentChunks(content string) []string {
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if normalized == "" {
		return nil
	}

	segments := strings.Split(normalized, "\n\n")
	if len(segments) == 1 {
		segments = strings.Split(normalized, "\n")
	}

	chunks := []string{}
	for _, segment := range segments {
		cleaned := strings.Join(strings.Fields(segment), " ")
		if cleaned == "" {
			continue
		}

		for _, chunk := range splitChunk(cleaned, knowledgeDocumentChunkSize) {
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
		}
	}

	return chunks
}

func splitChunk(content string, maxLength int) []string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil
	}

	if len(runes) <= maxLength {
		return []string{string(runes)}
	}

	chunks := []string{}
	start := 0
	for start < len(runes) {
		end := start + maxLength
		if end >= len(runes) {
			chunks = append(chunks, strings.TrimSpace(string(runes[start:])))
			break
		}

		splitAt := end
		for splitAt > start && !unicode.IsSpace(runes[splitAt-1]) {
			splitAt--
		}
		if splitAt == start {
			splitAt = end
		}

		chunks = append(chunks, strings.TrimSpace(string(runes[start:splitAt])))
		start = splitAt
		for start < len(runes) && unicode.IsSpace(runes[start]) {
			start++
		}
	}

	return chunks
}

func buildKnowledgeSnippet(content, query string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if normalized == "" {
		return ""
	}

	contentRunes := []rune(normalized)
	if len(contentRunes) <= knowledgeSnippetSize {
		return normalized
	}

	lowerContent := strings.ToLower(normalized)
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	matchIndex := strings.Index(lowerContent, lowerQuery)
	if matchIndex == -1 {
		return strings.TrimSpace(string(contentRunes[:knowledgeSnippetSize])) + "..."
	}

	matchRunes := []rune(normalized[:matchIndex])
	queryRunes := []rune(query)
	start := len(matchRunes) - knowledgeSnippetSize/3
	if start < 0 {
		start = 0
	}
	end := start + knowledgeSnippetSize
	if end < len(matchRunes)+len(queryRunes) {
		end = len(matchRunes) + len(queryRunes)
	}
	if end > len(contentRunes) {
		end = len(contentRunes)
	}
	if end-start > knowledgeSnippetSize && end == len(contentRunes) {
		start = max(0, end-knowledgeSnippetSize)
	}

	snippet := strings.TrimSpace(string(contentRunes[start:end]))
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(contentRunes) {
		snippet += "..."
	}
	return snippet
}

func escapeLikePattern(query string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(query)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

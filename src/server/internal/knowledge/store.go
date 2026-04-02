package knowledge

import (
	"context"
	"time"

	"oblivious/server/internal/auth"
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
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO knowledge_documents (id, knowledge_base_id, title, content, created_at, updated_at)
		SELECT $1, kb.id, $3, $4, $5, $5
		FROM knowledge_bases kb
		WHERE kb.workspace_id = $2 AND kb.id = $6
	`, documentID, workspaceID, title, content, now, knowledgeBaseID); err != nil {
		return KnowledgeDocument{}, err
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_bases
		SET document_count = document_count + 1, updated_at = $2
		WHERE workspace_id = $1 AND id = $3
	`, workspaceID, now, knowledgeBaseID); err != nil {
		return KnowledgeDocument{}, err
	}

	return KnowledgeDocument{
		Content:   content,
		ID:        documentID,
		Title:     title,
		UpdatedAt: now,
	}, nil
}

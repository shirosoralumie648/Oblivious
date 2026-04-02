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

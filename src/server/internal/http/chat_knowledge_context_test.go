package http

import (
	"context"
	"reflect"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/knowledge"
)

func TestChatKnowledgeContextProviderRetrievesBoundKnowledgeBasesWithTopCitations(t *testing.T) {
	retriever := &recordingKnowledgeContextRetriever{
		basesByKnowledgeBaseID: map[string]knowledge.KnowledgeBase{
			"kb_2": {ID: "kb_2", Name: "Operations"},
			"kb_5": {ID: "kb_5", Name: "Finance"},
		},
		resultsByKnowledgeBaseID: map[string][]knowledge.KnowledgeRetrievalResult{
			"kb_2": {
				{
					DocumentTitle: "Runbook.md",
					Similarity:    0.91,
					Snippet:       "Rollback requires staged deploys.",
					Source: knowledge.KnowledgeCitation{
						DocumentTitle:  "Runbook.md",
						MatchedSnippet: "Rollback requires staged deploys.",
						SourceURL:      "https://docs.example.com/runbook",
					},
				},
			},
			"kb_5": {
				{
					DocumentTitle: "Billing.md",
					Similarity:    0.96,
					Snippet:       "Invoices must be reconciled before launch.",
					Source: knowledge.KnowledgeCitation{
						DocumentTitle: "Billing.md",
					},
				},
			},
		},
	}
	provider := chatKnowledgeContextProvider{retriever: retriever}

	session := auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}}
	contexts, err := provider.RelevantKnowledge(context.Background(), session, []string{" kb_2 ", "", "kb_5", "kb_2"}, " rollback plan ", 5)
	if err != nil {
		t.Fatalf("RelevantKnowledge returned error: %v", err)
	}

	if len(contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %+v", contexts)
	}
	if contexts[0].KnowledgeBaseID != "kb_5" || contexts[0].KnowledgeBaseName != "Finance" {
		t.Fatalf("unexpected first context source: %+v", contexts[0])
	}
	if contexts[0].Content != "Invoices must be reconciled before launch." || contexts[0].DocumentTitle != "Billing.md" {
		t.Fatalf("unexpected first context payload: %+v", contexts[0])
	}
	if contexts[1].KnowledgeBaseID != "kb_2" || contexts[1].KnowledgeBaseName != "Operations" || contexts[1].Content != "Rollback requires staged deploys." || contexts[1].SourceURL != "https://docs.example.com/runbook" {
		t.Fatalf("unexpected second context payload: %+v", contexts[1])
	}
	if !reflect.DeepEqual(retriever.getCalls, []knowledgeContextGetCall{
		{knowledgeBaseID: "kb_2", session: session},
		{knowledgeBaseID: "kb_5", session: session},
	}) {
		t.Fatalf("unexpected knowledge base lookup calls: %+v", retriever.getCalls)
	}
	if !reflect.DeepEqual(retriever.calls, []knowledgeContextRetrievalCall{
		{
			knowledgeBaseID: "kb_2",
			options: knowledge.KnowledgeRetrievalOptions{
				Mode:  knowledge.KnowledgeRetrievalModeHybrid,
				Limit: 5,
			},
			query:   "rollback plan",
			session: session,
		},
		{
			knowledgeBaseID: "kb_5",
			options: knowledge.KnowledgeRetrievalOptions{
				Mode:  knowledge.KnowledgeRetrievalModeHybrid,
				Limit: 5,
			},
			query:   "rollback plan",
			session: session,
		},
	}) {
		t.Fatalf("unexpected retrieval calls: %+v", retriever.calls)
	}
}

func TestChatKnowledgeContextProviderReturnsEmptyForBlankQueryOrNoBindings(t *testing.T) {
	retriever := &recordingKnowledgeContextRetriever{}
	provider := chatKnowledgeContextProvider{retriever: retriever}
	session := auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}}

	for _, tc := range []struct {
		name             string
		knowledgeBaseIDs []string
		query            string
	}{
		{name: "blank query", knowledgeBaseIDs: []string{"kb_2"}, query: "   "},
		{name: "no bindings", knowledgeBaseIDs: nil, query: "rollback"},
	} {
		contexts, err := provider.RelevantKnowledge(context.Background(), session, tc.knowledgeBaseIDs, tc.query, 5)
		if err != nil {
			t.Fatalf("%s: RelevantKnowledge returned error: %v", tc.name, err)
		}
		if len(contexts) != 0 {
			t.Fatalf("%s: expected no contexts, got %+v", tc.name, contexts)
		}
	}

	if len(retriever.calls) != 0 {
		t.Fatalf("expected retriever not to be called, got %+v", retriever.calls)
	}
}

func TestKnowledgeResultsToChatContextsPreservesTraceabilityFields(t *testing.T) {
	contexts := knowledgeResultsToChatContexts(
		knowledge.KnowledgeBase{ID: "kb_2", Name: "Operations"},
		[]knowledge.KnowledgeRetrievalResult{
			{
				DocumentTitle: "Fallback title",
				Similarity:    0.91,
				Snippet:       "fallback snippet",
				Source: knowledge.KnowledgeCitation{
					ChunkID:            "chunk_7",
					ChunkIndex:         3,
					DocumentID:         "doc_9",
					DocumentTitle:      "Runbook.md",
					DocumentVersion:    "v4",
					HighlightPositions: []knowledge.KnowledgeHighlightPosition{{Start: 0, End: 8}},
					OriginalText:       "Rollback requires a staged deploy and incident owner.",
					PageNumber:         15,
					MatchedSnippet:     "Rollback requires a staged deploy.",
					SourceURL:          "https://docs.example.com/runbook",
				},
			},
		},
		5,
	)

	expected := []chat.KnowledgeContext{
		{
			ChunkID:            "chunk_7",
			ChunkIndex:         3,
			Content:            "Rollback requires a staged deploy.",
			DocumentID:         "doc_9",
			DocumentTitle:      "Runbook.md",
			DocumentVersion:    "v4",
			HighlightPositions: []chat.KnowledgeHighlightPosition{{Start: 0, End: 8}},
			KnowledgeBaseID:    "kb_2",
			KnowledgeBaseName:  "Operations",
			OriginalText:       "Rollback requires a staged deploy and incident owner.",
			PageNumber:         15,
			Score:              0.91,
			SourceURL:          "https://docs.example.com/runbook",
		},
	}
	if !reflect.DeepEqual(contexts, expected) {
		t.Fatalf("expected traceable knowledge context, got %+v", contexts)
	}
}

type knowledgeContextRetrievalCall struct {
	knowledgeBaseID string
	options         knowledge.KnowledgeRetrievalOptions
	query           string
	session         auth.Session
}

type knowledgeContextGetCall struct {
	knowledgeBaseID string
	session         auth.Session
}

type recordingKnowledgeContextRetriever struct {
	basesByKnowledgeBaseID   map[string]knowledge.KnowledgeBase
	calls                    []knowledgeContextRetrievalCall
	getCalls                 []knowledgeContextGetCall
	resultsByKnowledgeBaseID map[string][]knowledge.KnowledgeRetrievalResult
}

func (r *recordingKnowledgeContextRetriever) Get(ctx context.Context, session auth.Session, knowledgeBaseID string) (knowledge.KnowledgeBase, error) {
	r.getCalls = append(r.getCalls, knowledgeContextGetCall{
		knowledgeBaseID: knowledgeBaseID,
		session:         session,
	})
	return r.basesByKnowledgeBaseID[knowledgeBaseID], nil
}

func (r *recordingKnowledgeContextRetriever) RetrieveWithOptions(ctx context.Context, session auth.Session, knowledgeBaseID, query string, options knowledge.KnowledgeRetrievalOptions) ([]knowledge.KnowledgeRetrievalResult, error) {
	r.calls = append(r.calls, knowledgeContextRetrievalCall{
		knowledgeBaseID: knowledgeBaseID,
		options:         options,
		query:           query,
		session:         session,
	})
	return append([]knowledge.KnowledgeRetrievalResult(nil), r.resultsByKnowledgeBaseID[knowledgeBaseID]...), nil
}

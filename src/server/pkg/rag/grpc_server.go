package rag

import (
	"context"

	ragv1 "oblivious/server/internal/grpc/ragv1"
)

type Server struct {
	ragv1.UnimplementedRAGServiceServer
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) CreateKnowledgeBase(ctx context.Context, req *ragv1.CreateKnowledgeBaseRequest) (*ragv1.CreateKnowledgeBaseResponse, error) {
	return &ragv1.CreateKnowledgeBaseResponse{
		KbId:   "kb-" + req.Name,
		Status: "created",
	}, nil
}

func (s *Server) UploadDocument(ctx context.Context, req *ragv1.UploadDocumentRequest) (*ragv1.UploadDocumentResponse, error) {
	return &ragv1.UploadDocumentResponse{
		DocumentId: "doc-" + req.DocumentName,
		Status:     "uploaded",
	}, nil
}

func (s *Server) Retrieve(ctx context.Context, req *ragv1.RetrieveRequest) (*ragv1.RetrieveResponse, error) {
	return &ragv1.RetrieveResponse{
		Chunks: []*ragv1.RetrievedChunk{
			{
				DocumentId: "doc-1",
				Content:    "sample content for query: " + req.Query,
				Score:      0.95,
			},
		},
	}, nil
}

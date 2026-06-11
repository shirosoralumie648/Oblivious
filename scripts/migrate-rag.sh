#!/bin/bash
set -e

SUPABASE_URL="${SUPABASE_URL:-}"
SUPABASE_SERVICE_KEY="${SUPABASE_SERVICE_KEY:-}"

if [ -z "$SUPABASE_URL" ] || [ -z "$SUPABASE_SERVICE_KEY" ]; then
  echo "Error: SUPABASE_URL and SUPABASE_SERVICE_KEY must be set"
  exit 1
fi

psql "$SUPABASE_URL" <<SQL
-- Create knowledge_bases table
CREATE TABLE IF NOT EXISTS knowledge_bases (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create documents table
CREATE TABLE IF NOT EXISTS documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  knowledge_base_id UUID REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  embedding VECTOR(1536),
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create index on embeddings
CREATE INDEX IF NOT EXISTS documents_embedding_idx ON documents USING ivfflat (embedding vector_cosine_ops);

-- Enable RLS
ALTER TABLE knowledge_bases ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
SQL

echo "Migration complete"

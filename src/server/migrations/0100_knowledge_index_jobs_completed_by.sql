-- Preserve terminal worker identity for RAG indexing release evidence after
-- releasing the active job lease.

ALTER TABLE IF EXISTS knowledge_index_jobs
  ADD COLUMN IF NOT EXISTS completed_by TEXT NOT NULL DEFAULT '';

UPDATE knowledge_index_jobs
SET completed_by = locked_by
WHERE status = 'succeeded'
  AND COALESCE(completed_by, '') = ''
  AND COALESCE(locked_by, '') <> '';

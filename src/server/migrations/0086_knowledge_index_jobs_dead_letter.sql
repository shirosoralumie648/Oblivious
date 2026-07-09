-- Add retry exhaustion, lease ownership, and terminal dead-letter state for
-- durable Knowledge/RAG vector index jobs.

ALTER TABLE IF EXISTS knowledge_index_jobs
  ADD COLUMN IF NOT EXISTS max_attempts INTEGER NOT NULL DEFAULT 5,
  ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS locked_by TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

UPDATE knowledge_index_jobs
SET max_attempts = 5
WHERE max_attempts <= 0;

ALTER TABLE IF EXISTS knowledge_index_jobs
  DROP CONSTRAINT IF EXISTS knowledge_index_jobs_status_check;

ALTER TABLE IF EXISTS knowledge_index_jobs
  DROP CONSTRAINT IF EXISTS knowledge_index_jobs_max_attempts_check;

ALTER TABLE IF EXISTS knowledge_index_jobs
  ADD CONSTRAINT knowledge_index_jobs_status_check
  CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead_letter'));

ALTER TABLE IF EXISTS knowledge_index_jobs
  ADD CONSTRAINT knowledge_index_jobs_max_attempts_check
  CHECK (max_attempts > 0);

UPDATE knowledge_index_jobs
SET completed_at = updated_at
WHERE status = 'succeeded'
  AND completed_at IS NULL;

UPDATE knowledge_index_jobs
SET status = 'dead_letter',
    error = CASE
      WHEN error = '' THEN 'dead_letter: retry attempts exhausted'
      WHEN error LIKE 'dead_letter:%' THEN error
      ELSE 'dead_letter: ' || error
    END,
    locked_at = NULL,
    locked_by = '',
    available_at = updated_at,
    completed_at = COALESCE(completed_at, updated_at),
    updated_at = NOW()
WHERE status = 'failed'
  AND attempts >= max_attempts;

-- Allow durable Knowledge/RAG index jobs to outlive deleted SQL documents.

ALTER TABLE IF EXISTS knowledge_index_jobs
  DROP CONSTRAINT IF EXISTS knowledge_index_jobs_document_id_fkey;

ALTER TABLE IF EXISTS knowledge_index_jobs
  DROP CONSTRAINT IF EXISTS knowledge_index_jobs_operation_check;

ALTER TABLE IF EXISTS knowledge_index_jobs
  ADD CONSTRAINT knowledge_index_jobs_operation_check
  CHECK (operation IN ('upsert_document', 'delete_document'));

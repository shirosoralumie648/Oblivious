-- Relay pricing catalog used as the runtime billing authority.

CREATE TABLE IF NOT EXISTS relay_pricing_entries (
  id TEXT PRIMARY KEY,
  api_type TEXT NOT NULL,
  model TEXT NOT NULL,
  dimension TEXT NOT NULL,
  unit_cost DECIMAL(18,10) NOT NULL CHECK (unit_cost > 0),
  markup DECIMAL(10,4) NOT NULL DEFAULT 1 CHECK (markup > 0),
  currency TEXT NOT NULL DEFAULT 'quota',
  source TEXT NOT NULL DEFAULT 'operator',
  active BOOLEAN NOT NULL DEFAULT true,
  effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (api_type, model, dimension, effective_from)
);

CREATE INDEX IF NOT EXISTS idx_relay_pricing_entries_active
  ON relay_pricing_entries (model, api_type, dimension)
  WHERE active = true;

INSERT INTO relay_pricing_entries (id, api_type, model, dimension, unit_cost, markup, source, effective_from)
VALUES
  ('rpe_chat_gpt4o_prompt_v1', 'chat', 'gpt-4o', 'prompt_tokens', 0.002, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_chat_gpt4o_completion_v1', 'chat', 'gpt-4o', 'completion_tokens', 0.008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_chat_gpt4o_total_v1', 'chat', 'gpt-4o', 'total_tokens', 0.008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_chat_gpt4omini_prompt_v1', 'chat', 'gpt-4o-mini', 'prompt_tokens', 0.0002, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_chat_gpt4omini_completion_v1', 'chat', 'gpt-4o-mini', 'completion_tokens', 0.0008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_chat_gpt4omini_total_v1', 'chat', 'gpt-4o-mini', 'total_tokens', 0.0008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_responses_gpt4o_prompt_v1', 'responses', 'gpt-4o', 'prompt_tokens', 0.002, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_responses_gpt4o_completion_v1', 'responses', 'gpt-4o', 'completion_tokens', 0.008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_responses_gpt4o_total_v1', 'responses', 'gpt-4o', 'total_tokens', 0.008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_responses_gpt4omini_prompt_v1', 'responses', 'gpt-4o-mini', 'prompt_tokens', 0.0002, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_responses_gpt4omini_completion_v1', 'responses', 'gpt-4o-mini', 'completion_tokens', 0.0008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_responses_gpt4omini_total_v1', 'responses', 'gpt-4o-mini', 'total_tokens', 0.0008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_completions_gpt4o_prompt_v1', 'completions', 'gpt-4o', 'prompt_tokens', 0.002, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_completions_gpt4o_completion_v1', 'completions', 'gpt-4o', 'completion_tokens', 0.008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_completions_gpt4o_total_v1', 'completions', 'gpt-4o', 'total_tokens', 0.008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_completions_gpt4omini_prompt_v1', 'completions', 'gpt-4o-mini', 'prompt_tokens', 0.0002, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_completions_gpt4omini_completion_v1', 'completions', 'gpt-4o-mini', 'completion_tokens', 0.0008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_completions_gpt4omini_total_v1', 'completions', 'gpt-4o-mini', 'total_tokens', 0.0008, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_embeddings_gpt4o_prompt_v1', 'embeddings', 'gpt-4o', 'prompt_tokens', 0.0001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_embeddings_gpt4omini_prompt_v1', 'embeddings', 'gpt-4o-mini', 'prompt_tokens', 0.00001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_images_gpt4o_count_v1', 'images_generations', 'gpt-4o', 'image_count', 0.004, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_images_gpt4omini_count_v1', 'images_generations', 'gpt-4o-mini', 'image_count', 0.0004, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_audio_speech_gpt4o_seconds_v1', 'audio_speech', 'gpt-4o', 'audio_seconds', 0.000015, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_audio_speech_gpt4omini_seconds_v1', 'audio_speech', 'gpt-4o-mini', 'audio_seconds', 0.0000015, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_audio_stt_gpt4o_seconds_v1', 'audio_transcriptions', 'gpt-4o', 'audio_seconds', 0.0001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_audio_stt_gpt4omini_seconds_v1', 'audio_transcriptions', 'gpt-4o-mini', 'audio_seconds', 0.00001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_audio_translate_gpt4o_seconds_v1', 'audio_translations', 'gpt-4o', 'audio_seconds', 0.0001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_audio_translate_gpt4omini_seconds_v1', 'audio_translations', 'gpt-4o-mini', 'audio_seconds', 0.00001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_moderation_gpt4o_prompt_v1', 'moderations', 'gpt-4o', 'prompt_tokens', 0.0001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_moderation_gpt4o_total_v1', 'moderations', 'gpt-4o', 'total_tokens', 0.0001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_moderation_gpt4omini_prompt_v1', 'moderations', 'gpt-4o-mini', 'prompt_tokens', 0.00001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_moderation_gpt4omini_total_v1', 'moderations', 'gpt-4o-mini', 'total_tokens', 0.00001, 1, 'initial_catalog', '2026-07-02T00:00:00Z'),
  ('rpe_files_storage_bytes_v1', 'files', '', 'storage_bytes', 0.000000001, 1, 'initial_catalog', '2026-07-02T00:00:00Z')
ON CONFLICT (id) DO UPDATE SET
  api_type = EXCLUDED.api_type,
  model = EXCLUDED.model,
  dimension = EXCLUDED.dimension,
  unit_cost = EXCLUDED.unit_cost,
  markup = EXCLUDED.markup,
  source = EXCLUDED.source,
  active = true,
  updated_at = NOW();

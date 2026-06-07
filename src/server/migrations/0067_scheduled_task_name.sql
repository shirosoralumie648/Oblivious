ALTER TABLE scheduled_tasks
ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT 'Scheduled task';

ALTER TABLE scheduled_tasks
ALTER COLUMN name DROP DEFAULT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'scheduled_tasks_name_not_blank'
    ) THEN
        ALTER TABLE scheduled_tasks
        ADD CONSTRAINT scheduled_tasks_name_not_blank CHECK (btrim(name) <> '');
    END IF;
END $$;

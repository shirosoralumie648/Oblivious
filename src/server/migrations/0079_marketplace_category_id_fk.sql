-- Marketplace category integrity
-- Publishing now accepts stable category IDs, so legacy slug/blank values must
-- be normalized before the database enforces the categories(id) relationship.

INSERT INTO categories (id, name, slug, display_order)
SELECT 'cat_productivity', 'Productivity', 'productivity', 6
WHERE NOT EXISTS (
    SELECT 1 FROM categories WHERE id = 'cat_productivity'
)
AND NOT EXISTS (
    SELECT 1 FROM categories WHERE slug = 'productivity'
);

UPDATE published_agents pa
SET category_id = c.id
FROM categories c
WHERE pa.category_id = c.slug
  AND pa.category_id <> c.id;

WITH fallback_category AS (
    SELECT id
    FROM categories
    WHERE id = 'cat_productivity' OR slug = 'productivity'
    ORDER BY
        CASE
            WHEN id = 'cat_productivity' THEN 0
            WHEN slug = 'productivity' THEN 1
            ELSE 2
        END,
        display_order ASC,
        id ASC
    LIMIT 1
),
first_category AS (
    SELECT id
    FROM categories
    ORDER BY display_order ASC, id ASC
    LIMIT 1
),
chosen_category AS (
    SELECT COALESCE(
        (SELECT id FROM fallback_category),
        (SELECT id FROM first_category)
    ) AS id
)
UPDATE published_agents pa
SET category_id = chosen_category.id
FROM chosen_category
WHERE chosen_category.id IS NOT NULL
  AND (
      pa.category_id IS NULL
      OR btrim(pa.category_id) = ''
      OR NOT EXISTS (
          SELECT 1
          FROM categories c
          WHERE c.id = pa.category_id
      )
  );

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM published_agents
        WHERE category_id IS NULL OR btrim(category_id) = ''
    ) THEN
        RAISE EXCEPTION 'published_agents.category_id contains null or blank values after backfill';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM published_agents pa
        WHERE NOT EXISTS (
            SELECT 1
            FROM categories c
            WHERE c.id = pa.category_id
        )
    ) THEN
        RAISE EXCEPTION 'published_agents.category_id contains values that do not reference categories.id';
    END IF;
END
$$;

ALTER TABLE IF EXISTS published_agents
    ALTER COLUMN category_id SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'published_agents_category_id_fkey'
          AND conrelid = 'published_agents'::regclass
    ) THEN
        ALTER TABLE published_agents
            ADD CONSTRAINT published_agents_category_id_fkey
            FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_published_agents_category_id
    ON published_agents(category_id);

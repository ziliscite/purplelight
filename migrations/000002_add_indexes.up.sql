CREATE INDEX IF NOT EXISTS anime_title_idx ON anime USING GIN (to_tsvector('simple', title));

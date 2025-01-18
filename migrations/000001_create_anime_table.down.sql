-- Drop the anime_tag table first (if it exists)
DROP TABLE IF EXISTS anime_tag;

-- Drop the anime, tag tables
DROP TABLE IF EXISTS tag;
DROP TABLE IF EXISTS anime;

-- Drop the enums in reverse order of creation
DROP TYPE IF EXISTS season;
DROP TYPE IF EXISTS status;
DROP TYPE IF EXISTS anime_type;

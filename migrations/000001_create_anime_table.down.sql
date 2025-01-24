-- Drop the anime_tags table first, since it depends on the anime table
DROP TABLE IF EXISTS anime_tags;

-- Now you can safely drop the anime, tag tables, and the enums
DROP TABLE IF EXISTS anime;
DROP TABLE IF EXISTS tag;

-- Finally, drop the custom types (enums)
DROP TYPE IF EXISTS season;
DROP TYPE IF EXISTS status;
DROP TYPE IF EXISTS anime_type;

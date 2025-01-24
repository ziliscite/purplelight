-- Define AnimeType enum
CREATE TYPE anime_type AS ENUM ('TV', 'Movie', 'OVA', 'ONA', 'Special');

-- Define Status enum
CREATE TYPE status AS ENUM ('Ongoing', 'Finished', 'Upcoming');

-- Define Season enum
CREATE TYPE season AS ENUM ('Spring', 'Summer', 'Fall', 'Winter');

-- Create the Anime table using these enums
CREATE TABLE anime (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL UNIQUE,
    type anime_type NOT NULL,
    episodes INTEGER DEFAULT NULL,
    status status NOT NULL,
    season season DEFAULT NULL,
    year INTEGER DEFAULT NULL,
    duration INTEGER DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1
);

-- Create the Tag table
CREATE TABLE tag (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Link the tags to each anime in both tables
CREATE TABLE anime_tags (
    anime_id INTEGER REFERENCES anime(id) ON DELETE CASCADE,
    tag_id INTEGER REFERENCES tag(id) ON DELETE CASCADE,
    PRIMARY KEY (anime_id, tag_id)
);



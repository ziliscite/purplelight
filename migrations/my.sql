-- Define AnimeType enum
CREATE TYPE anime_type AS ENUM ('TV', 'Movie', 'OVA', 'ONA', 'Special');

-- Define Status enum
CREATE TYPE status AS ENUM ('Ongoing', 'Finished', 'Upcoming');

-- Define Season enum
CREATE TYPE season AS ENUM ('Spring', 'Summer', 'Fall', 'Winter');

-- Create the Anime table using these enums
CREATE TABLE anime (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    type anime_type NOT NULL,
    episodes INTEGER NOT NULL,
    status status NOT NULL,
    season season NOT NULL,
    year INTEGER NOT NULL,
    tags VARCHAR(255)[] NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    version INTEGER NOT NULL
);

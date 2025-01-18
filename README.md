# Chapter 6

We’re going to explore how to use SQL migrations to create the table (and more generally, manage database schema changes throughout the project).
- The high-level principles behind SQL migrations and why they are useful.
- How to use the command-line migrate tool to programmatically manage changes to your database schema.

## Overview of SQL Migrations
#### Very high-level concept
- For every change that you want to make to your database schema (like creating a table, adding a column, or removing an unused index) you create a pair of migration files. One file is the ‘up’ migration which contains the SQL statements necessary to implement the change, and the other is a ‘down’ migration which contains the SQL statements to reverse (or roll-back) the change.
- Each pair of migration files is numbered sequentially, usually 0001, 0002, 0003... or with a Unix timestamp, to indicate the order in which migrations should be applied to a database.
- You use some kind of tool or script to execute or rollback the SQL statements in the sequential migration files against your database. The tool keeps track of which migrations have already been applied, so that only the necessary SQL statements are actually executed.

#### Schema benefits
- The database schema (along with its evolution and changes) is completely described by the ‘up’ and ‘down’ SQL migration files. And because these are just regular files containing some SQL statements, they can be included and tracked alongside the rest of your code in a version control system.
- It’s possible to replicate the current database schema precisely on another machine by running the necessary ‘up’ migrations. This is a big help when you need to manage and synchronize database schemas in different environments (development, testing, production, etc.).
- It’s possible to roll-back database schema changes if necessary by applying the appropriate ‘down’ migrations.

### Installing the migrate tool
macOS
```shell
$ brew install golang-migrate
```

Linux & Windows
```shell
$ cd /tmp
$ curl -L https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz | tar xvz
$ mv migrate ~/go/bin/
```

## Working with SQL Migrations
The first thing we need to do is generate a pair of migration files using the migrate create command
```shell
migrate create -seq -ext sql -dir migrations create_anime_table
```

At the moment these two new files are completely empty. Let’s edit the ‘up’ migration file to contain the necessary CREATE TABLE statement for our movies table, like so:
```sql
-- Define AnimeType enum
CREATE TYPE anime_type AS ENUM ('TV', 'Movie', 'OVA', 'ONA', 'Special');

-- Define Status enum
CREATE TYPE status AS ENUM ('Ongoing', 'Finished', 'Upcoming');

-- Define Season enum
CREATE TYPE season AS ENUM ('Spring', 'Summer', 'Fall', 'Winter');

-- Create the Anime table using these enums
CREATE TABLE anime (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    type anime_type NOT NULL,
    episodes INTEGER DEFAULT NULL,
    status status NOT NULL,
    season season DEFAULT NULL,
    year INTEGER DEFAULT NULL,
    duration INTEGER DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 0
);

-- Create the Tag table
CREATE TABLE tag (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Link the tags to each anime in both tables
CREATE TABLE anime_tags (
    anime_id INTEGER REFERENCES anime(id) ON DELETE CASCADE,
    tag_id INTEGER REFERENCES tag(id) ON DELETE CASCADE,
    PRIMARY KEY (anime_id, tag_id)
);
```

Alright, let’s move on to the ‘down’ migration and add the SQL statements needed to reverse the ‘up’ migration that we just wrote.
```sql
-- Drop the anime_tag table first (if it exists)
DROP TABLE IF EXISTS anime_tag;

-- Drop the anime, tag tables
DROP TABLE IF EXISTS tag;
DROP TABLE IF EXISTS anime;

-- Drop the enums in reverse order of creation
DROP TYPE IF EXISTS season;
DROP TYPE IF EXISTS status;
DROP TYPE IF EXISTS anime_type;
```

// mine isn't using below cause different implementation
While we are at it, let’s also create a second pair of migration files containing CHECK constraints to enforce some of our business rules at the database-level. 
Specifically, we want to make sure that the runtime value is always greater than zero, the year value is between 1888 and the current year, and the genres array always contains between 1 and 5 items.
```shell
$ migrate create -seq -ext=.sql -dir=./migrations add_movies_check_constraints
```
And then add the following SQL statements to add and drop the CHECK constraints respectively:
```sql
ALTER TABLE movies ADD CONSTRAINT movies_runtime_check CHECK (runtime >= 0);

ALTER TABLE movies ADD CONSTRAINT movies_year_check CHECK (year BETWEEN 1888 AND date_part('year', now()));

ALTER TABLE movies ADD CONSTRAINT genres_length_check CHECK (array_length(genres, 1) BETWEEN 1 AND 5);
```
```sql
ALTER TABLE movies DROP CONSTRAINT IF EXISTS movies_runtime_check;

ALTER TABLE movies DROP CONSTRAINT IF EXISTS movies_year_check;

ALTER TABLE movies DROP CONSTRAINT IF EXISTS genres_length_check;
```

When we insert or update data in our movies table, if any of these checks fail our database driver will return an error similar to this:

```shell
pq: new row for relation "movies" violates check constraint "movies_year_check"
```

## Executing Migrations
```shell
migrate -path=./migrations -database=$PURPLELIGHT_DB_DSN up
```

// mine just this
```shell
PS C:\Users\manzi\GolandProjects\purplelight> migrate -path migrations -database postgres://purplelight:Neige@localhost/purplelight?sslmode=disable up
1/u create_anime_table (62.1426ms)
```

At this point, it’s worth opening a connection to your database and listing the tables with the \dt meta command:
```shell
purplelight=> \dt
                List of relations                                                                                  
 Schema |       Name        | Type  |    Owner
--------+-------------------+-------+-------------
 public | anime             | table | purplelight
 public | anime_tags        | table | purplelight
 public | schema_migrations | table | purplelight
 public | tag               | table | purplelight                                                                  
(4 rows) 
```
The schema_migrations table is automatically generated by the migrate tool and used to keep track of which migrations have been applied.

The version column here indicates that our migration files up to (and including) number 2 in the sequence have been executed against the database. The value of the dirty column is false, which indicates that the migration files were cleanly executed without any errors and the SQL statements they contain were successfully applied in full.

### Migrate to specific version
```shell
migrate -path=./migrations -database=$EXAMPLE_DSN goto 1
```

### Down Migrations
You can use the down command to roll-back by a specific number of migrations. For example, to rollback the most recent migration you would run:
```shell
migrate -path=./migrations -database =$EXAMPLE_DSN down 1
```

### Migration Errors
When you run a migration that contains an error, all SQL statements up to the erroneous one will be applied and then the migrate tool will exit with a message describing the error.
```shell
$ migrate -path=./migrations -database=$EXAMPLE_DSN up
1/u create_foo_table (36.6328ms)
2/u create_bar_table (71.835442ms)
error: migration failed: syntax error at end of input in line 0: CREATE TABLE (details: pq: syntax error at end of input)
```

In turn, this means that the database is in an unknown state as far as the migrate tool is concerned.

Accordingly, the version field in the schema_migrations field will contain the number for the failed migration and the dirty field will be set to true. At this point, if you run another migration (even a ‘down’ migration) you will get an error message similar to this:
```shell
Dirty database version {X}. Fix and force version.
```

What you need to do is investigate the original error and figure out if the migration file which failed was partially applied. If it was, then you need to manually roll-back the partially applied migration.

Once that’s done, then you must also ‘force’ the version number in the schema_migrations table to the correct value. For example, to force the database version number to 1 you should use the force command like so:
```shell
migrate -path=./migrations -database=$EXAMPLE_DSN force 1
```

### Remote migrations
The migrate tool also supports reading migration files from remote sources including Amazon S3 and GitHub repositories.
```shell
$ migrate -source="s3://<bucket>/<path>" -database=$EXAMPLE_DSN up
$ migrate -source="github://owner/repo/path#ref" -database=$EXAMPLE_DSN up
$ migrate -source="github://user:personal-access-token@owner/repo/path#ref" -database=$EXAMPLE_DSN up
```

### Running migrations on application startup
It is also possible to use the golang-migrate/migrate Go package (not the command-line tool) to automatically execute your database migrations on application start up.
```go
package main

import (
    "context"      
    "database/sql" 
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/golang-migrate/migrate/v4"                   // New import
    "github.com/golang-migrate/migrate/v4/database/postgres" // New import
    _ "github.com/golang-migrate/migrate/v4/source/file"     // New import
    _ "github.com/lib/pq"
)

func main() {
    ...

    db, err := openDB(cfg)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
    defer db.Close()

    logger.Info("database connection pool established")

    migrationDriver, err := postgres.WithInstance(db, &postgres.Config{})
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }

    migrator, err := migrate.NewWithDatabaseInstance("file:///path/to/your/migrations", "postgres", migrationDriver)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }

    err = migrator.Up()
    if err != nil && err != migrate.ErrNoChange {
        logger.Error(err.Error())
        os.Exit(1)
    }
    
    logger.Info("database migrations applied")

    ...
}

Although this works — and it might initially seem appealing — tightly coupling the execution of migrations with your application source code can potentially be limiting and problematic in the longer term.

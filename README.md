# Chapter 5

In this section we're setting up db (postgres) and config. Including:
- How to install and set up PostgreSQL on your local machine.
- How to use the psql interactive tool to create databases, PostgreSQL extensions and user accounts.
- How to initialize a database connection pool in Go and configure its settings to improve performance and stability.


### Setting up PostgresSQL

Install

Mac
```shell
$ brew install postgresql@15 
```

Linux
```shell
$ sudo apt install postgresql
```

Windows
```shell
> choco install postgresql
```

Connect to psql
```shell
sudo -u postgres psql

postgres=# SELECT current_user;
 current_user 
--------------
 postgres
(1 row)
```

Creating and connecting to a new postgresql database
```shell
PS C:\Users\manzi\GolandProjects\purplelight> psql -U toy
Password for user toy: 
psql (16.4)
WARNING: Console code page (437) differs from Windows code page (1252)
         8-bit characters might not work correctly. See psql reference
         page "Notes for Windows users" for details.
Type "help" for help.

toy=# CREATE DATABASE purplelight;
CREATE DATABASE
toy=# \c purplelight
You are now connected to database "purplelight" as user "toy".
purplelight=#
```

Creating new purplelight user, without superuser permissions
```shell
purplelight=# CREATE ROLE purplelight WITH LOGIN PASSWORD 'Neige';
CREATE ROLE
```

Postgres extensions  
This project going to use the citext extension. This adds a case-insensitive character string type to PostgreSQL.
```shell
purplelight=# CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION
```

Connect to new user
```shell
PS C:\Users\manzi\GolandProjects\purplelight> psql --host=localhost --dbname=purplelight --username=purplelight
Password for user purplelight: 
psql (16.4)
WARNING: Console code page (437) differs from Windows code page (1252)
         8-bit characters might not work correctly. See psql reference
         page "Notes for Windows users" for details.
Type "help" for help.

purplelight=>
```

Optimizing Postgres settings  
we can improve our db by tweaking values in `postgresql.conf`  
to access it, run the following command:
```shell
$ sudo -u postgres psql -c 'SHOW config_file;'
               config_file               
-----------------------------------------
 /etc/postgresql/15/main/postgresql.conf
(1 row)
```

[This](https://www.enterprisedb.com/postgres-tutorials/how-tune-postgresql-memory) provides a good introduction to some of the most important PostgreSQL settings


### Connecting to PostgresSQL
This book uses `github.com/lib/pq@v1`
```go
> go get github.com/lib/pq@v1
```

To connect to the database we’ll also need a data source name (DSN), which is basically a string that contains the necessary connection parameters.
`postgres://purplelight:Neige@localhost/purplelight` could also use `?sslmode=disable`

When establish connection pool
- We want the DSN to be configurable at runtime, so we will pass it to the application using a command-line flag rather than hard-coding it. For simplicity during development, we’ll use the DSN above as the default value for the flag.
- In our cmd/api/main.go file we’ll create a new openDB() helper function. In this helper we’ll use the sql.Open() function to establish a new sql.DB connection pool, then — because connections to the database are established lazily as and when needed for the first time — we will also need to use the db.PingContext() method to actually create a connection and verify that everything is set up correctly.

To do it
```go
import (
    "context"      // New import
    "database/sql" // New import
	...

    // Import the pq driver so that it can register itself with the database/sql 
    // package. Note that we alias this import to the blank identifier, to stop the Go 
    // compiler complaining that the package isn't being used.
    _ "github.com/lib/pq"
)
```

```go
// Add a db struct field to hold the configuration settings for our database connection
// pool. For now this only holds the DSN, which we will read in from a command-line flag.
type config struct {
    port int
    env  string
    db   struct {
        dsn string
    }
}

// Read the DSN value from the db-dsn command-line flag into the config struct. We
// default to using our development DSN if no flag is provided.
flag.StringVar(&cfg.db.dsn, "db-dsn", "postgres://purplelight:Neige@localhost/purplelight", "PostgreSQL DSN")
flag.Parse()
```

```go
// Call the openDB() helper function (see below) to create the connection pool,
// passing in the config struct. If this returns an error, we log it and exit the
// application immediately.
db, err := openDB(cfg)
if err != nil {
    logger.Error(err.Error())
    os.Exit(1)
}

// Defer a call to db.Close() so that the connection pool is closed before the
// main() function exits.
defer db.Close()

// Also log a message to say that the connection pool has been successfully 
// established.
logger.Info("database connection pool established")
```

```go
// The openDB() function returns a sql.DB connection pool.
func openDB(cfg config) (*sql.DB, error) {
    // Use sql.Open() to create an empty connection pool, using the DSN from the config
    // struct.
    db, err := sql.Open("postgres", cfg.db.dsn)
    if err != nil {
        return nil, err
    }

    // Create a context with a 5-second timeout deadline.
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Use PingContext() to establish a new connection to the database, passing in the
    // context we created above as a parameter. If the connection couldn't be
    // established successfully within the 5 second deadline, then this will return an
    // error. If we get this error, or any other, we close the connection pool and 
    // return the error.
    err = db.PingContext(ctx)
    if err != nil {
        db.Close()
        return nil, err
    }

    // Return the sql.DB connection pool.
    return db, nil
}
```

To not hardcode the DSN, we create a new environment variable  
Add it to either $HOME/.profile or $HOME/.bashrc files
```shell
ziliscite@Lx:~$ nano $HOME/.profile

# add export PURPLELIGHT_DB_DSN='postgres://purplelight:Neige@localhost/purplelight?sslmode=disable'

ziliscite@Lx:~$ source $HOME/.profile
ziliscite@Lx:~$ echo $PURPLELIGHT_DB_DSN
postgres://purplelight:Neige@localhost/purplelight?sslmode=disable
```

// Here I'll just use .env file

### Configuring the Database Connection Pool
In this chapter we’re going to go in-depth — explaining how the connection pool works behind the scenes, and exploring the settings we can use to change and optimize its behavior.

An sql.DB pool contains two types of connections — ‘in-use’ connections and ‘idle’ connections.  
> A connection is marked as in-use when you are using it to perform a database task, such as executing a SQL statement or querying rows, and when the task is complete the connection is then marked as idle.

The SetMaxOpenConns() method allows you to set an upper MaxOpenConns limit on the number of ‘open’ connections (in-use + idle connections) in the pool. By default, the number of open connections is unlimited.  
PostgreSQL has a hard limit of 100 open connections  
If this hard limit is hit, it will return a "sorry, too many clients already" error.  
The hard limit on open connections can be changed in your postgresql.conf file using the max_connections setting.  
The benefit of setting a MaxOpenConns limit is that it acts as a very rudimentary throttle, and prevents the database from being swamped by a huge number of tasks all at once.  

> If the MaxOpenConns limit is reached, and all connections are in-use, then any further database tasks will be forced to wait until a connection becomes free and marked as idle. In the context of our API, the user’s HTTP request could ‘hang’ indefinitely while waiting for a free connection. So to mitigate this, it’s important to always set a timeout on database tasks using a context.Context object.

The SetMaxIdleConns() method sets an upper MaxIdleConns limit on the number of idle connections in the pool. By default, the maximum number of idle connections is 2.  
In theory, allowing more idle connections will improve performance because it makes it less likely that a new connection needs to be established from scratch — therefore helping to save resources.  
But it’s also important to realize that keeping an idle connection alive comes at a cost. It takes up memory.  
MaxIdleConns limit should always be less than or equal to MaxOpenConns. Go enforces this and will automatically reduce the MaxIdleConns limit if necessary.  

The SetConnMaxLifetime() method sets the ConnMaxLifetime limit — the maximum length of time that a connection can be reused for. By default, there’s no maximum lifetime and connections will be reused forever.  
- This doesn’t guarantee that a connection will exist in the pool for a whole hour; it’s possible that a connection will become unusable for some reason and be automatically closed before then.
- A connection can still be in use more than one hour after being created — it just cannot start to be reused after that time.
- This isn’t an idle timeout. The connection will expire one hour after it was first created — not one hour after it last became idle.
- Once every second Go runs a background cleanup operation to remove expired connections from the pool.

> If you do decide to set a ConnMaxLifetime on your pool, it’s important to bear in mind the frequency at which connections will expire (and subsequently be recreated). For example, if you have 100 open connections in the pool and a ConnMaxLifetime of 1 minute, then your application can potentially kill and recreate up to 1.67 connections (on average) every second. You don’t want the frequency to be so great that it ultimately hinders performance.

The SetConnMaxIdleTime() method sets the ConnMaxIdleTime limit. This works in a very similar way to ConnMaxLifetime, except it sets the maximum length of time that a connection can be idle for before it is marked as expired. By default there’s no limit.  
If we set ConnMaxIdleTime to 1 hour, for example, any connections that have sat idle in the pool for 1 hour since last being used will be marked as expired and removed by the background cleanup operation.  
This setting is really useful because it means that we can set a relatively high limit on the number of idle connections in the pool, but periodically free-up resources by removing any idle connections that we know aren’t really being used anymore.

Putting it into practice
- As a rule of thumb, you should explicitly set a MaxOpenConns value. This should be comfortably below any hard limits on the number of connections imposed by your database and infrastructure, and you may also want to consider keeping it fairly low to act as a rudimentary throttle.
- For this project we’ll set a MaxOpenConns limit of 25 connections. I’ve found this to be a reasonable starting point for small-to-medium web applications and APIs, but ideally you should tweak this value for your hardware depending on the results of benchmarking and load-testing.
- In general, higher MaxOpenConns and MaxIdleConns values will lead to better performance. But the returns are diminishing, and you should be aware that having a too-large idle connection pool (with connections that are not frequently re-used) can actually lead to reduced performance and unnecessary resource consumption.
- Because MaxIdleConns should always be less than or equal to MaxOpenConns, we’ll also limit MaxIdleConns to 25 connections for this project.
- To mitigate the risk from point 2 above, you should generally set a ConnMaxIdleTime value to remove idle connections that haven’t been used for a long time. In this project we’ll set a ConnMaxIdleTime duration of 15 minutes.
- It’s probably OK to leave ConnMaxLifetime as unlimited, unless your database imposes a hard limit on connection lifetime, or you need it specifically to facilitate something like gracefully swapping databases. Neither of those things apply in this project, so we’ll leave this as the default unlimited setting.

```go
// Read the connection pool settings from command-line flags into the config struct.
// Notice that the default values we're using are the ones we discussed above?
flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

...

// Set the maximum number of open (in-use + idle) connections in the pool. Note that
// passing a value less than or equal to 0 will mean there is no limit.
db.SetMaxOpenConns(cfg.db.maxOpenConns)

// Set the maximum number of idle connections in the pool. Again, passing a value
// less than or equal to 0 will mean there is no limit.
db.SetMaxIdleConns(cfg.db.maxIdleConns)

// Set the maximum idle timeout for connections in the pool. Passing a duration less
// than or equal to 0 will mean that connections are not closed due to their idle time. 
db.SetConnMaxIdleTime(cfg.db.maxIdleTime)
```




# Chapter 19. Building, Versioning and Quality Control
In this section of the book we’re going shift our focus from writing code to managing and maintaining our project, and take steps to help automate common tasks and prepare our API for deployment.

- Use a makefile to automate common tasks in your project, such as creating and executing migrations.
- Carry out quality control checks of your code using the go vet and staticcheck tools.
- Vendor third-party packages, in case they ever become unavailable in the future.
- Build and run executable binaries for your applications, reduce their size, and cross-compile binaries for different platforms.
- Burn-in a version number and build time to your application when building the binary.
- Leverage Git to generate automated version numbers as part of your build process.

## Creating and Using Makefiles
To install it

Windows:
```shell
choco install make
```

macOS:
```shell
brew install make
```

Linux:
```shell
sudo apt install make
```

### A simple makefile
We’ll start simple, and then build things up step-by-step.

A makefile is essentially a text file which contains one or more `rules` that the `make` utility can run. Each rule has a `target` and contains a sequence of sequential `commands` which are executed when the rule is run.

Makefile rules have the following structure:
```shell
# comment (optional)
target: 
	command
	command
	...
```

> Please note that each command in a makefile rule must start with a tab character, not spaces.

// Yeah, a bit of frustration on my side of why my Makefile didnt work back then...

Create a rule which executes the go run ./cmd/api command to run our API application.
```shell
run:
	go run ./cmd/api
```

Make sure that the Makefile is saved, and then you can execute a specific rule by running $ make <target> from your terminal.
```shell
C:\Users\manzi\GolandProjects\purplelight>make run
go run ./cmd/api
time=2025-01-31T22:08:27.476+07:00 level=INFO msg="database connection pool established"
time=2025-01-31T22:08:27.477+07:00 level=INFO msg="starting server" addr=:4000 env=development
```

When we type make run, the make utility looks for a file called Makefile or makefile in the current directory and then executes the commands associated with the run target.

By default, make echoes commands in the terminal output. We can see that in the code above where the first line in the output is the echoed command go run ./cmd/api. If you want, it’s possible to suppress commands from being echoed by prefixing them with the @ character.

### Environment variables
When we execute a make rule, every environment variable that is available to make when it starts is transformed into a make variable with the same name and value. We can then access these variables using the syntax ${VARIABLE_NAME} in our makefile.

To illustrate this, let’s create two additional rules — a psql rule for connecting to our database and an up rule to execute our database migrations.

```shell
C:\Users\manzi\GolandProjects\purplelight>make psql
psql (16.4)
WARNING: Console code page (437) differs from Windows code page (1252)
         8-bit characters might not work correctly. See psql reference
         page "Notes for Windows users" for details.
Type "help" for help.

purplelight=>
```

### Passing arguments
The make utility also allows you to pass named arguments when executing a particular rule.

Add a migration rule to our makefile to generate a new pair of migration files.
```shell
run:
	go run ./cmd/api

psql:
	psql ${GREENLIGHT_DB_DSN}

migration:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

up:
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up
```

```shell
$ make migration name=create_example_table
Creating migration files for create_example_table ...
migrate create -seq -ext=.sql -dir=./migrations create_example_table
/home/alex/Projects/greenlight/migrations/000007_create_example_table.up.sql
/home/alex/Projects/greenlight/migrations/000007_create_example_table.down.sql
```

### Namespacing targets
Namespacing your target names to provide some differentiation between rules and help organize the file.

For example, in a large makefile rather than having the target name up it would be clearer to give it the name db/migrations/up instead.

Update our target names to use some sensible namespaces.
```shell
run/api:
	go run ./cmd/api

db/psql:
	psql ${GREENLIGHT_DB_DSN}

db/migrations/new:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

db/migrations/up:
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up
```

And you should be able to execute the rules by typing the full target name when running make.
```shell
$ make run/api 
go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
```

A nice feature of using the / character as the namespace separator is that you get tab completion in the terminal when typing target names. For example, if you type make db/migrations/ and then hit tab on your keyboard the remaining targets under the namespace will be listed.
```shell
$ make db/migrations/
new  up
```

### Prerequisite targets and asking for confirmation
It’s also possible to specify prerequisite targets.

```shell
target: prerequisite-target-1 prerequisite-target-2 ...
	command
	command
	...
```

When you specify a prerequisite target for a rule, the corresponding commands for the prerequisite targets will be run before executing the actual target commands.

To do this, we’ll create a new confirm target which asks the user Are you sure? [y/N] and exits with an error if they do not enter y. 

```shell
# Create the new confirm target.
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

run/api:
	go run ./cmd/api

db/psql:
	psql ${GREENLIGHT_DB_DSN}

db/migrations/new:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

# Include it as prerequisite.
db/migrations/up: confirm
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up
```

Essentially, what happens here is that we ask the user Are you sure? [y/N] and then read the response. We then use the code [ $${ans:-N} = y ] to evaluate the response — this will return true if the user enters y and false if they enter anything else. If a command in a makefile returns false, then make will stop running the rule and exit with an error message — essentially stopping the rule in its tracks.
```shell
$ make db/migrations/up 
Are you sure? [y/N] y
Running up migrations...
migrate -path ./migrations -database postgres://greenlight:pa55word@localhost/greenlight up
no change
```

### Displaying help information
Another small thing that we can do to make our makefile more user-friendly is to include some comments and help functionality.

We’ll create a new help rule which parses the makefile itself, extracts the help text from the comments using sed, formats them into a table and then displays them to the user.

```shell
## help: print this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

## run/api: run the cmd/api application
run/api:
	go run ./cmd/api

## db/psql: connect to the database using psql
db/psql:
	psql ${GREENLIGHT_DB_DSN}

## db/migrations/new name=$1: create a new database migration
db/migrations/new:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

## db/migrations/up: apply all up database migrations
db/migrations/up: confirm
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up
```

And if you now execute the help target
```shell
$ make help
Usage: 
  help                        print this help message
  run/api                     run the cmd/api application
  db/psql                     connect to the database using psql
  db/migrations/new name=$1   create a new database migration
  db/migrations/up            apply all up database migrations 
```

also, positioning the help rule as the first thing in the Makefile is a deliberate move. If you run make without specifying a target then it will default to executing the first rule in the file.
```shell
$ make
Usage: 
  help                        print this help message
  run/api                     run the cmd/api application
  db/psql                     connect to the database using psql
  db/migrations/new name=$1   create a new database migration
  db/migrations/up            apply all up database migrations 
```

### Phony targets
We’ve been using make to execute actions, but another (and arguably, the primary) purpose of make is to help create files on disk where the name of a target is the name of a file being created by the rule.

If you’re using make primarily to execute actions, like we are, then this can cause a problem if there is a file in your project directory with the same path as a target name.

If you want, you can demonstrate this problem by creating a file called ./run/api in the root of your project directory
```shell
mkdir run && touch run/api
```

And then, if you execute make run/api, instead of our API application starting up you’ll get the following message:
```shell
$ make run/api 
make: 'run/api' is up to date. 
```

Because we already have a file on disk at ./run/api, the make tool considers this rule to have already been executed and so returns the message that we see above without taking any further action.

To work around this, we can declare our makefile targets to be phony targets

> A phony target is one that is not really the name of a file; rather it is just a name for a rule to be executed.

To declare a target as phony
```shell
.PHONY: target
target: prerequisite-target-1 prerequisite-target-2 ...
	command
	command
	...
```

Go ahead and update out Makefile so that all our rules have phony targets
```shell
## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	go run ./cmd/api

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	psql ${GREENLIGHT_DB_DSN}

## db/migrations/new name=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

## db/migrations/up: apply all up database migrations
.PHONY: db/migrations/up
db/migrations/up: confirm
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up
```

If you run make run/api again now, it should now correctly recognize this as a phony target and execute the rule for us

You might think that it’s only necessary to declare targets as phony if you have a conflicting file name, but in practice `not declaring a target as phony when it actually is can lead to bugs or confusing behavior`.

## Managing Environment Variables
Using the make run/api command to run our API application opens up an opportunity to tweak our command-line flags, and remove the default value for our database DSN from the main.go file.
```go
func main() {
    var cfg config

    flag.IntVar(&cfg.port, "port", 4000, "API server port")
    flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

    // Use the empty string "" as the default value for the db-dsn command-line flag,
    // rather than os.Getenv("GREENLIGHT_DB_DSN") like we were previously.
    flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")

    ...
}
```

Instead, we can update our makefile so that the DSN value from the GREENLIGHT_DB_DSN environment variable is passed in as part of the rule.
```shell
## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	go run ./cmd/api -db-dsn=${GREENLIGHT_DB_DSN}
```

This is a small change but a really nice one, because it means that the default configuration values for our application no longer change depending on the operating environment. The command-line flag values passed at runtime are the sole mechanism for configuring our application settings, and there are still no secrets hard-coded in our project files.

During development running our application remains nice and easy — all we need to do is type make `run/api`.

### Using a .envrc file
If you like, you could also remove the GREENLIGHT_DB_DSN environment variable from your $HOME/.profile or $HOME/.bashrc files, and store it in a .envrc file in the root of your project directory instead.
```dotenv
export GREENLIGHT_DB_DSN=postgres://greenlight:pa55word@localhost/greenlight
```

You can then use a tool like direnv to automatically load the variables from the .envrc file into your current shell, or alternatively, you can add an include command at the top of your Makefile to load them instead.
```shell
# Include variables from the .envrc file
include .envrc

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
```

This approach is particularly convenient in projects where you need to make frequent changes to your environment variables, because it means that you can just edit the .envrc file without needing to reboot your computer or run source after each change.

Another nice benefit of this approach is that it provides a degree of separation between variables if you’re working on multiple projects on the same machine.

## Quality Controlling Code
We’re going to add new audit and tidy rules to our Makefile to check, test and tidy up our codebase automatically.

It’s useful to have rules like these that you can routinely run before you commit changes into your version control system or build any binaries.

The `audit` rule won’t make any changes to our codebase, but will simply report any problems.

- Use the go mod tidy -diff command to check if the go.mod and go.sum files are out of date and need to be fixed (which you can do by running go mod tidy).
- Use the go mod verify command to check that the dependencies on your computer (located in your module cache located at $GOPATH/pkg/mod) haven’t been changed since they were downloaded and that they match the cryptographic hashes in your go.sum file. Running this helps ensure that the dependencies being used are the exact ones that you expect.
- Use the go vet ./... command to check all .go files in the project directory. The go vet tool runs a variety of analyzers which carry out static analysis of your code and warn you about things which might be wrong but won’t be picked up by the compiler — such as unreachable code, unnecessary assignments, and badly-formed build tags.
- Use the third-party `staticcheck` tool to carry out some additional static analysis checks.
- Use the go test -race -vet=off ./... command to run all tests in the project directory. By default, go test automatically executes a small subset of the go vet checks before running any tests, so to avoid duplication we’ll use the -vet=off flag to turn this off. The -race flag enables Go’s race detector, which can help pick up certain classes of race conditions while tests are running.

In contrast, the `tidy` rule will actually make changes to the codebase. It will:
- Use the go fmt ./... command to format all .go files in the project directory, according to the Go standard. This will reformat files ‘in place’ and output the names of any changed files.
- Use the go mod tidy command to prune any unused dependencies from the go.mod and go.sum files, and add any missing dependencies.

Install the `staticcheck` tool
```shell
$ go install honnef.co/go/tools/cmd/staticcheck@latest
go: downloading honnef.co/go/tools v0.1.3
go: downloading golang.org/x/tools v0.1.0
go: downloading github.com/BurntSushi/toml v0.3.1
go: downloading golang.org/x/sys v0.0.0-20210119212857-b64e53b001e4
go: downloading golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
go: downloading golang.org/x/mod v0.6.0
$ which staticcheck
/home/alex/go/bin/staticcheck
```

// In my machine, the command wasn't `which`, but `staticcheck`

Give it a try.
```shell
PS C:\Users\manzi\GolandProjects\purplelight> make tidy
'Formatting .go files...'
go fmt ./...
cmd\api\anime_utils.go
cmd\api\users.go
internal\data\duration.go
internal\data\errors.go
internal\repository\errors.go
'Tidying module dependencies...'
go mod tidy

PS C:\Users\manzi\GolandProjects\purplelight> make audit
'Checking module dependencies'
go mod tidy -diff
go mod verify
all modules verified
'Vetting code...'
go vet ./...
staticcheck ./...
cmd\api\middlewares.go:293:25: func (*application).enableAllCORS is unused (U1000)
internal\repository\anime.go:203:24: strings.Title has been deprecated since Go 1.18 and an alternative h
as been available since Go 1.0: The rule Title uses for word boundaries does not handle Unicode punctuation properly. Use golang.org/x/text/cases instead.  (SA1019)
internal\repository\anime.go:207:12: unnecessary use of fmt.Sprintf (S1039)
internal\repository\anime.go:210:11: unnecessary use of fmt.Sprintf (S1039)
internal\repository\tags.go:34:26: func AnimeRepository.upsertTag is unused (U1000)
internal\repository\tags.go:91:26: func AnimeRepository.getAnimeTags is unused (U1000)
```

## Module Proxies and Vendoring
One of the risks of using third-party packages in your Go code is that the package repository may cease to be available.

### Module proxies
Go supports module proxies (also known as module mirrors) by default. These are services which mirror source code from the original, authoritative, repositories (such as those hosted on GitHub, GitLab or BitBucket).

Go ahead and run the go env command on your machine:
```shell
$ go env
GO111MODULE=""
GOARCH="amd64"
GOBIN=""
GOCACHE="/home/alex/.cache/go-build"
...
GOPROXY="https://proxy.golang.org,direct"
...
```

The important thing to look at here is the GOPROXY setting, which contains a comma-separated list of module mirrors.

The URL `https://proxy.golang.org` that we see here points to a module mirror maintained by the Go team at Google, containing copies of the source code from tens of thousands of open-source Go packages.

Whenever you fetch a package using the go command — either with go get or one of the go mod * commands — it will first attempt to retrieve the source code from this mirror.

Using a module mirror as the first fetch location has a few benefits:

- The https://proxy.golang.org module mirror typically stores packages long-term, thereby providing a degree of protection in case the original repository disappears from the internet.
- It’s not possible to override or delete a package once it’s stored in the https://proxy.golang.org module mirror. This can help prevent any bugs or problems which might arise if a package author (or an attacker) releases an edited version of the package with the same version number.
- Fetching modules from the https://proxy.golang.org mirror can be much faster than getting them from the authoritative repositories.

### Vendoring
Go’s module mirror functionality is great, but it isn’t a silver bullet for all developers and all projects.

You should also be aware that the default proxy.golang.org module mirror doesn’t absolutely guarantee that it will store a copy of the module forever.

Additionally, if you need to come back to a ‘cold’ codebase in 5 or 10 years’ time, will the proxy.golang.org module mirror still be available?

So, for these reasons, it can still be sensible to vendor your project dependencies using the `go mod vendor` command. Vendoring dependencies in this way basically stores a complete copy of the source code for third-party packages in a `vendor` folder in your project.

We’ll start by adapting our make tidy rule to also call the go mod verify and go mod vendor commands
```shell
## tidy: format all .go files, and tidy and vendor module dependencies
.PHONY: tidy
tidy:
	@echo 'Formatting .go files...'
	go fmt ./...
	@echo 'Tidying module dependencies...'
	go mod tidy
	@echo 'Verifying and vendoring module dependencies...'
	go mod verify
	go mod vendor
```

- The go mod tidy command will make sure the go.mod and go.sum files list all the necessary dependencies for our project (and no unnecessary ones).
- The go mod verify command will verify that the dependencies stored in your module cache (located on your machine at $GOPATH/pkg/mod) match the cryptographic hashes in the go.sum file.
- The go mod vendor command will then copy the necessary source code from your module cache into a new vendor directory in your project root.

Try it out. Once that’s completed, you should see that a new vendor directory has been created containing copies of all the source code along with a modules.txt file.
```shell
$ tree -L 3 ./vendor/
./vendor/
├── github.com
│   ├── go-mail
│   │   └── mail
│   ├── julienschmidt
│   │   └── httprouter
│   └── lib
│       └── pq
├── golang.org
│   └── x
│       ├── crypto
│       └── time
├── gopkg.in
│   └── alexcesaro
│       └── quotedprintable.v3
└── modules.txt
```

Now, when you run a command such as go run, go test or go build, the go tool will recognize the presence of a vendor folder and the dependency code in the vendor folder will be used — rather than the code in the module cache on your local machine.

Also take a quick look in the `vendor/modules.txt` file
```go
# github.com/go-mail/mail/v2 v2.3.0
## explicit
github.com/go-mail/mail/v2
# github.com/julienschmidt/httprouter v1.3.0
## explicit; go 1.7
github.com/julienschmidt/httprouter
# github.com/lib/pq v1.10.9
## explicit; go 1.13
github.com/lib/pq
github.com/lib/pq/oid
github.com/lib/pq/scram
# github.com/tomasen/realip v0.0.0-20180522021738-f0c99a92ddce
## explicit
github.com/tomasen/realip
# golang.org/x/crypto v0.26.0
## explicit; go 1.17
golang.org/x/crypto/bcrypt
golang.org/x/crypto/blowfish
# golang.org/x/time v0.6.0
## explicit
golang.org/x/time/rate
# gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc
## explicit
gopkg.in/alexcesaro/quotedprintable.v3
# gopkg.in/mail.v2 v2.3.1
## explicit
```

This vendor/modules.txt file is essentially a manifest of the vendored packages and their version numbers.

### Vendoring new dependencies
In the next section of the book, we’re going to deploy our API application to the internet with Caddy as a reverse-proxy in-front of it. This means that, as far as our API is concerned, all the requests it receives will be coming from a single IP address (the one running the Caddy instance). In turn, that will cause problems for our rate limiter middleware which limits access based on IP address.

Fortunately, like most other reverse proxies, Caddy adds an X-Forwarded-For header to each request. This header will contain the real IP address for the client.

Checking for the presence of this header can be done, but its easier to use the `realip` package. This package retrieves the client IP address from any X-Forwarded-For or X-Real-IP headers, falling back to use r.RemoteAddr if neither of them are present.
```shell
go get github.com/tomasen/realip@latest
```

Update the `rateLimit()` middleware to use this package
```go
func (app *application) rateLimit(next http.Handler) http.Handler {

    ...

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if app.config.limiter.enabled {
            // Use the realip.FromRequest() function to get the client's real IP address.
            ip := realip.FromRequest(r)

            mu.Lock()

            if _, found := clients[ip]; !found {
                clients[ip] = &client{
                    limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
                }
            }

            clients[ip].lastSeen = time.Now()

            if !clients[ip].limiter.Allow() {
                mu.Unlock()
                app.rateLimitExceededResponse(w, r)
                return
            }

            mu.Unlock()
        }

        next.ServeHTTP(w, r)
    })
}
```

If you try to run the API application again now, you should receive an error message similar to this:
```shell
$ make run/api 
go: inconsistent vendoring in /home/alex/Projects/greenlight:
        github.com/tomasen/realip@v0.0.0-20180522021738-f0c99a92ddce: is explicitly 
            required in go.mod, but not marked as explicit in vendor/modules.txt

        To ignore the vendor directory, use -mod=readonly or -mod=mod.
        To sync the vendor directory, run:
                go mod vendor
make: *** [Makefile:24: run/api] Error 1
```

Essentially what’s happening here is that Go is looking for the github.com/tomasen/realip package in our vendor directory, but at the moment that package doesn’t exist in there.

To solve this, you’ll need to run the make tidy command.

### The ./… pattern
Most of the go tools support the ./... wildcard pattern, like go fmt ./..., go vet ./... and go test ./.... This pattern matches the current directory and all sub-directories, excluding the vendor directory.

Generally speaking, this is useful because it means that we’re not formatting, vetting or testing the code in our vendor directory unnecessarily — and our make audit rule won’t fail due to any problems that might exist within those vendored packages.

## Building Binaries
So far we’ve been running our API using the go run command (or more recently, make run/api). But in this chapter we’re going to focus on explaining how to build an executable binary that you can distribute and run on other machines without needing the Go toolchain installed.

To build a binary:
```shell
$ go build -o=./bin/api ./cmd/api
```

When we run this command, go build will compile the cmd/api package (and any dependent packages) into files containing machine code, and then link these together to an form executable binary.

Add a new build/api rule
```shell
## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -o=./bin/api ./cmd/api
```

Once that’s done, go ahead and execute the make build/api rule.
```shell
$ make build/api 
Building cmd/api...
go build -o=./bin/api ./cmd/api
$ ls -l ./bin/
total 10228
-rwxrwxr-x 1 alex alex 10470419 Apr 18 16:05 api
```

### Reducing binary size
If you take a closer look at the executable binary you’ll see that it weighs in at 10470419 bytes (about 10.5MB).

// mine's slightly larger
```shell
Mode                 LastWriteTime         Length Name
----                 -------------         ------ ----
-a----          2/1/2025  12:41 AM       18308096 purplelight.exe
```

It’s possible to reduce the binary size by around 25% by instructing the Go linker to strip symbol tables and DWARF debugging information from the binary. We can do this as part of the go build command by using the linker flag -ldflags="-s"
```shell
## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags='-s' -o=./bin/api ./cmd/api
```

It’s important to be aware that stripping out this information will make it harder to debug an executable using a tool like Delve or gdb. But, generally, it’s not often that you’ll need to do this — and there’s even an open proposal from Rob Pike to make omitting DWARF information the default behavior of the linker in the future.

### Cross-compilation
By default, the go build command will output a binary suitable for use on your local machine’s operating system and architecture. But it also supports cross-compilation, so you can generate a binary suitable for use on a different machine.

To see a list of all the operating system/architecture combinations that Go supports, you can run the `go tool dist list` command
```shell
$ go tool dist list
aix/ppc64
android/386
android/amd64
android/arm
android/arm64
darwin/amd64
...
```

And you can specify the operating system and architecture that you want to create the binary for by setting GOOS and GOARCH environment variables when running go build.
```shell
GOOS=linux GOARCH=amd64 go build {args}
```

Let’s update our make build/api rule so that it creates two binaries — one for use on your local machine, and another for deploying to the Ubuntu Linux server.
```shell
## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags='-s' -o=./bin/api ./cmd/api
	GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o=./bin/linux_amd64/api ./cmd/api
```

As a general rule, you probably don’t want to commit your Go binaries into version control alongside your source code as they will significantly inflate the size of your repository.

### Build caching
`go build` command caches build output in the Go build cache. This cached output will be reused again in future builds where appropriate, which can significantly speed up the overall build time for your application.

You should also be aware that the build cache does not automatically detect any changes to C libraries that your code imports with cgo. So, if you’ve changed a C library since the last build, you’ll need to use the -a flag to force all packages to be rebuilt when running go build.

## Managing and Automating Version Numbers
Right at the start of this book, we hard-coded the version number for our application as the constant "1.0.0" in the cmd/api/main.go file.

We’re going take steps to make it easier to view and manage this version number, and also explain how you can generate version numbers automatically based on Git commits and integrate them into your application.

### Displaying the version number
Let’s start by updating our application so that we can easily check the version number by running the binary with a -version command-line flag.

Conceptually, this is fairly straightforward to implement. We need to define a boolean version command-line flag, check for this flag on startup, and then print out the version number and exit the application if necessary.
```go
func main() {
    var cfg config

    flag.IntVar(&cfg.port, "port", 4000, "API server port")
    flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

    flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")
    
    flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
    flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
    flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

    flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
    flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
    flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")

    flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
    flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
    flag.StringVar(&cfg.smtp.username, "smtp-username", "a7420fc0883489", "SMTP username")
    flag.StringVar(&cfg.smtp.password, "smtp-password", "e75ffd0a3aa5ec", "SMTP password")
    flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Greenlight <no-reply@greenlight.alexedwards.net>", "SMTP sender")

    flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
        cfg.cors.trustedOrigins = strings.Fields(val)
        return nil
    })

    // Create a new version boolean flag with the default value of false.
    displayVersion := flag.Bool("version", false, "Display version and exit")

    flag.Parse()

    // If the version flag value is true, then print out the version number and
    // immediately exit.
    if *displayVersion {
        fmt.Printf("Version:\t%s\n", version)
        os.Exit(0)
    }

    ...
}
```

It prints out the version number and then exits
```shell
$ make build/api 
Building cmd/api...
go build -ldflags="-s" -o="./bin/api" ./cmd/api
GOOS=linux GOARCH=amd64 go build -ldflags="-s" -o="./bin/linux_amd64/api" ./cmd/api

$ ./bin/api -version
Version:        1.0.0
```

### Automated version numbering with Git
Since version 1.18, Go now embeds version control information in your executable binaries when you `run go build` on a main package that is tracked with `Git`, `Mercurial`, `Fossil`, or `Bazaar`.

There are two ways to access this version control information — either by using the `go version -m` command on your binary, or from within your application code itself by calling `debug.ReadBuildInfo()`.

If you look at your commit history using the git log command, you’ll see the hash for this commit.

```shell
PS C:\Users\manzi\GolandProjects\purplelight> git log 
commit 4197036a6383e7b8e67c62811256e9a041b88c07 (HEAD -> chapter-19)
Author: Zil <Manzil.akbar@gmail.com>
Date:   Sat Feb 1 00:42:23 2025 +0700

    makefile
```

Next `run make build` again to generate a new binary and then use the go version -m command on it.
```shell
$ make build/api 
Building cmd/api...
go build -ldflags="-s" -o=./bin/api ./cmd/api
GOOS=linux GOARCH=amd64 go build -ldflags="-s" -o=./bin/linux_amd64/api ./cmd/api

$ go version -m ./bin/api 
./bin/api: go1.23.0
        path    greenlight.alexedwards.net/cmd/api
        mod     greenlight.alexedwards.net      (devel)
        dep     github.com/go-mail/mail/v2      v2.3.0  h1:wha99yf2v3cpUzD1V9ujP404Jbw2uEvs+rBJybkdYcw=
        dep     github.com/julienschmidt/httprouter     v1.3.0  h1:U0609e9tgbseu3rBINet9P48AI/D3oJs4dN7jwJOQ1U=
        dep     github.com/lib/pq       v1.10.9 h1:YXG7RB+JIjhP29X+OtkiDnYaXQwpS4JEWq7dtCCRUEw=
        dep     github.com/tomasen/realip       v0.0.0-20180522021738-f0c99a92ddce      h1:fb190+cK2Xz/dvi9Hv8eCYJYvIGUTN2/KLq1pT6CjEc=
        dep     golang.org/x/crypto     v0.26.0 h1:RrRspgV4mU+YwB4FYnuBoKsUapNIL5cohGAmSH3azsw=
        dep     golang.org/x/time       v0.6.0  h1:eTDhh4ZXt5Qf0augr54TN6suAUudPcawVZeIAPU7D4U=
        build   -buildmode=exe
        build   -compiler=gc
        build   -ldflags="-s"
        build   CGO_ENABLED=1
        build   CGO_CFLAGS=
        build   CGO_CPPFLAGS=
        build   CGO_CXXFLAGS=
        build   CGO_LDFLAGS=
        build   GOARCH=amd64
        build   GOOS=linux
        build   GOAMD64=v1
        build   vcs=git
        build   vcs.revision=3f5ab2cbaaf4bf7c936d03a1984d4abc08e8c6d3
        build   vcs.time=2023-09-10T06:37:03Z
        build   vcs.modified=true
```

The output from `go version -m` shows us some interesting information about the binary.

- vcs=git tells us that the version control system being used is Git.
- vcs.revision is the hash for the latest Git commit.
- vcs.time is the time that this commit was made.
- vcs.modified tells us whether the code tracked by the Git repository has been modified since the commit was made. A value of false indicates that the code has not been modified, meaning that the binary was built using the exact code from the vcs.revision commit. A value of true indicates that the version control repository was ‘dirty’ when the binary was built — and the code used to build the binary may not be the exact code from the vcs.revision commit.

Let’s leverage this and adapt our main.go file so that the version value is set to the Git commit hash, rather than the hardcoded constant "1.0.0".

Create a small internal/vcs package which generates a version number for our application based on the commit hash from vcs.revision plus an optional -dirty suffix if vcs.modified=true.
- Call the debug.ReadBuildInfo() function. This will return a debug.BuildInfo struct which contains essentially the same information that we saw when running the go version -m command.
- Loop through the debug.BuildInfo.Settings field to extract the vcs.revision and vcs.modified values.

File: internal/vcs/vcs.go
```go
func Version() string {
    var revision string
    var modified bool

    bi, ok := debug.ReadBuildInfo()
    if ok {
        for _, s := range bi.Settings {
            switch s.Key {
            case "vcs.revision":
                revision = s.Value
            case "vcs.modified":
                if s.Value == "true" {
                    modified = true
                }
            }
        }
    }

    if modified {
        return fmt.Sprintf("%s-dirty", revision)
    }

    return revision
}
```

File: cmd/api/main.go
```go
// Make version a variable (rather than a constant) and set its value to vcs.Version().
var (
    version = vcs.Version()
)
```

Then build and run it with the -version flag
```shell
$ ./bin/api -version
Version:        59bdb76fda0c15194ce18afae5d4875237f05ea9-dirty
```

> Version control information is only embedded by default when you run go build. It is never embedded when you use go run, and is only embedded when using go test if you use the -buildvcs=true flag.

Our application version number now aligns with the commit history in our Git repository, meaning that it’s easy for us to identify exactly what code a particular binary contains or a running application is using. All we need to do is run the binary with the -version flag, or call the healthcheck endpoint, and then cross-reference the version number against the Git repository history.

### Including commit time in the version number
```go
func Version() string {
    var (
        time     string
        revision string
        modified bool
    )

    bi, ok := debug.ReadBuildInfo()
    if ok {
        for _, s := range bi.Settings {
            switch s.Key {
            case "vcs.time":
                time = s.Value
            case "vcs.revision":
                revision = s.Value
            case "vcs.modified":
                if s.Value == "true" {
                    modified = true
                }
            }
        }
    }

    if modified {
        return fmt.Sprintf("%s-%s-dirty", time, revision)
    }

    return fmt.Sprintf("%s-%s", time, revision)
}
```

Making that change would result in version numbers that look similar to this:

`2022-04-30T10:16:24Z-1c9b6ff48ea800acdf4f5c6f5c3b62b98baf2bd7-dirty`

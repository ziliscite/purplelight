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




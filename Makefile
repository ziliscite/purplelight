# Include variables from the .envrc file
include .envrc

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## confirm: ask the user if they want to proceed
.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	@echo "Running purplelight..."
	@go run ./cmd/api -db-dsn=${PURPLELIGHT_DSN} -smtp-username=${SMTP_USERNAME} -smtp-password=${SMTP_PASSWORD}

## build/api: build the cmd/api application and run it
.PHONY: build/api
build/api:
	@echo "Building purplelight..."
	@go build -ldflags -s -o ./bin/purplelight.exe ./cmd/api
	@set GOOS=linux&& set GOARCH=amd64&& go build -ldflags -s -o ./bin/linux_amd64/purplelight ./cmd/api

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	@psql ${PURPLELIGHT_DSN}

## migrations/new name=$1: create a new database migration
.PHONY: migrations/new
migrations/new: confirm
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext .sql -dir ./migrations ${name}

## migrations/up: apply all up database migrations
.PHONY: migrations/up
migrations/up: confirm
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${PURPLELIGHT_DSN} up

## migrations/down: apply all down database migrations
.PHONY: migrations/down
migrations/down: confirm
	@echo 'Running down migrations...'
	migrate -path ./migrations -database ${PURPLELIGHT_DSN} down

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format all .go files and tidy module dependencies
.PHONY: tidy
tidy:
	@echo 'Formatting .go files...'
	go fmt ./...
	@echo 'Tidying module dependencies...'
	go mod tidy
	go mod verify

## audit: run quality control checks
.PHONY: audit
audit:
	@echo 'Checking module dependencies'
	go mod tidy -diff
	go mod verify
	@echo 'Vetting code...'
	go vet ./...
	staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

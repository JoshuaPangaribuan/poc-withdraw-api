GO ?= go
GOOSE ?= $(GO) run github.com/pressly/goose/v3/cmd/goose@latest
SQLC ?= sqlc
DOCKER_COMPOSE ?= docker compose
PSQL ?= psql

SERVICE ?= wallet

DB_HOST ?= localhost
DB_USER ?= admin
DB_PASSWORD ?= secret
DB_SSLMODE ?= disable

DB_AUTH_PORT ?= 5432
DB_AUTH_NAME ?= auth_db
DB_WALLET_PORT ?= 5433
DB_WALLET_NAME ?= wallet_db
DB_INQUIRY_PORT ?= $(DB_AUTH_PORT)
DB_INQUIRY_NAME ?= inquiry_db

ifeq ($(SERVICE),auth)
DB_PORT ?= $(DB_AUTH_PORT)
DB_NAME ?= $(DB_AUTH_NAME)
DB_COMPOSE_SERVICE ?= postgres-auth
else ifeq ($(SERVICE),wallet)
DB_PORT ?= $(DB_WALLET_PORT)
DB_NAME ?= $(DB_WALLET_NAME)
DB_COMPOSE_SERVICE ?= postgres-wallet
else
$(error unsupported SERVICE '$(SERVICE)'; expected auth or wallet)
endif

GOOSE_DRIVER ?= postgres
GOOSE_DBSTRING ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
GOOSE_DIR ?= db/migrations/$(SERVICE)
SEED_DIR ?= db/seeds/$(SERVICE)
GOOSE_DBSTRING_SINGLE ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_INQUIRY_PORT)/$(DB_INQUIRY_NAME)?sslmode=$(DB_SSLMODE)
GOOSE_DBSTRING_AUTH ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_AUTH_PORT)/$(DB_AUTH_NAME)?sslmode=$(DB_SSLMODE)
GOOSE_DBSTRING_WALLET ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_WALLET_PORT)/$(DB_WALLET_NAME)?sslmode=$(DB_SSLMODE)

BIN_DIR ?= bin
BIN ?=
APP_NAME ?= withdraw-api

.DEFAULT_GOAL := help

.PHONY: help setup run test build fmt mocks sqlc-generate infra-up infra-down db-shell migrate-up migrate-down migrate-status migrate-reset migrate-create seed create-inquiry-db migrate-up-single seed-single setup-single migrate-up-multi seed-multi setup-multi

help:
	@printf "%-20s %s\n" "setup" "Create local config files from examples"
	@printf "%-20s %s\n" "run BIN=..." "Run from main.go with BIN=inquiry|withdraw (default: all)"
	@printf "%-20s %s\n" "test" "Run unit tests"
	@printf "%-20s %s\n" "build BIN=..." "Build from main.go; embed default BIN=inquiry|withdraw (default: all)"
	@printf "%-20s %s\n" "fmt" "Run go fmt"
	@printf "%-20s %s\n" "mocks" "Generate mockery mocks under internal/mock"
	@printf "%-20s %s\n" "sqlc-generate" "Generate SQLC code"
	@printf "%-20s %s\n" "infra-up" "Start local infra (Postgres)"
	@printf "%-20s %s\n" "infra-down" "Stop local infra (Postgres)"
	@printf "%-20s %s\n" "db-shell SERVICE=..." "Open psql shell for auth|wallet"
	@printf "%-20s %s\n" "migrate-up SERVICE=..." "Apply all migrations for auth|wallet"
	@printf "%-20s %s\n" "migrate-down" "Rollback one migration"
	@printf "%-20s %s\n" "migrate-status" "Show migration status"
	@printf "%-20s %s\n" "migrate-reset" "Reset all migrations"
	@printf "%-20s %s\n" "migrate-create SERVICE=... name=..." "Create a new migration for auth|wallet"
	@printf "%-20s %s\n" "seed SERVICE=..." "Run SQL seed files for auth|wallet"
	@printf "%-20s %s\n" "setup-single" "Create inquiry_db, run all migrations, and seed for single instance"
	@printf "%-20s %s\n" "setup-multi" "Run auth+wallet migrations and seeds for multi instance"

setup:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "created .env from .env.example"; \
	else \
		echo ".env already exists, skipping"; \
	fi
	@if [ ! -f config.yaml ]; then \
		cp config.yaml.example config.yaml; \
		echo "created config.yaml from config.yaml.example"; \
	else \
		echo "config.yaml already exists, skipping"; \
	fi
	@if [ ! -f .env.inquiry ]; then \
		cp .env.inquiry.example .env.inquiry; \
		echo "created .env.inquiry from .env.inquiry.example"; \
	else \
		echo ".env.inquiry already exists, skipping"; \
	fi
	@if [ ! -f .env.withdraw ]; then \
		cp .env.withdraw.example .env.withdraw; \
		echo "created .env.withdraw from .env.withdraw.example"; \
	else \
		echo ".env.withdraw already exists, skipping"; \
	fi
	@if [ ! -f config.inquiry.yaml ]; then \
		cp config.inquiry.yaml.example config.inquiry.yaml; \
		echo "created config.inquiry.yaml from config.inquiry.yaml.example"; \
	else \
		echo "config.inquiry.yaml already exists, skipping"; \
	fi
	@if [ ! -f config.withdraw.yaml ]; then \
		cp config.withdraw.yaml.example config.withdraw.yaml; \
		echo "created config.withdraw.yaml from config.withdraw.yaml.example"; \
	else \
		echo "config.withdraw.yaml already exists, skipping"; \
	fi

run:
	$(GO) run . --bin=$(BIN)

test:
	$(GO) test ./...

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "-X main.defaultBin=$(BIN)" -o $(BIN_DIR)/$(APP_NAME) .

fmt:
	$(GO) fmt ./...

mocks:
	$(GO) run github.com/vektra/mockery/v2@v2.53.3

sqlc-generate:
	$(SQLC) generate

infra-up:
	$(DOCKER_COMPOSE) -f docker-compose-infra.yaml up -d

infra-down:
	$(DOCKER_COMPOSE) -f docker-compose-infra.yaml down

db-shell:
	@if command -v $(PSQL) >/dev/null 2>&1; then \
		PGPASSWORD="$(DB_PASSWORD)" $(PSQL) -h "$(DB_HOST)" -p "$(DB_PORT)" -U "$(DB_USER)" -d "$(DB_NAME)"; \
	else \
		$(DOCKER_COMPOSE) -f docker-compose-infra.yaml exec $(DB_COMPOSE_SERVICE) psql -U "$(DB_USER)" -d "$(DB_NAME)"; \
	fi

migrate-up:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(GOOSE_DBSTRING)" up

migrate-down:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(GOOSE_DBSTRING)" down

migrate-status:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(GOOSE_DBSTRING)" status

migrate-reset:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(GOOSE_DBSTRING)" reset

migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "usage: make migrate-create SERVICE=auth|wallet name=add_table"; \
		exit 1; \
	fi
	$(GOOSE) -dir $(GOOSE_DIR) create $(name) sql

seed:
	$(GOOSE) -dir $(SEED_DIR) -no-versioning $(GOOSE_DRIVER) "$(GOOSE_DBSTRING)" up

create-inquiry-db:
	@$(DOCKER_COMPOSE) -f docker-compose-infra.yaml exec -T postgres-auth psql -U "$(DB_USER)" -d postgres -tA -v ON_ERROR_STOP=1 -c "SELECT 1 FROM pg_database WHERE datname='$(DB_INQUIRY_NAME)'" | grep -q 1 || \
		$(DOCKER_COMPOSE) -f docker-compose-infra.yaml exec -T postgres-auth psql -U "$(DB_USER)" -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE $(DB_INQUIRY_NAME);"

migrate-up-single:
	$(GOOSE) -dir db/migrations $(GOOSE_DRIVER) "$(GOOSE_DBSTRING_SINGLE)" up

seed-single:
	$(GOOSE) -dir db/seeds -no-versioning $(GOOSE_DRIVER) "$(GOOSE_DBSTRING_SINGLE)" up

setup-single: create-inquiry-db migrate-up-single seed-single

migrate-up-multi:
	$(GOOSE) -dir db/migrations/auth $(GOOSE_DRIVER) "$(GOOSE_DBSTRING_AUTH)" up
	$(GOOSE) -dir db/migrations/wallet $(GOOSE_DRIVER) "$(GOOSE_DBSTRING_WALLET)" up

seed-multi:
	$(GOOSE) -dir db/seeds/auth -no-versioning $(GOOSE_DRIVER) "$(GOOSE_DBSTRING_AUTH)" up
	$(GOOSE) -dir db/seeds/wallet -no-versioning $(GOOSE_DRIVER) "$(GOOSE_DBSTRING_WALLET)" up

setup-multi: migrate-up-multi seed-multi

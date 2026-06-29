SHELL := /bin/bash

COMPOSE ?= docker compose
ENV_FILE ?= .env.docker
APP_PORT ?= 8080
DB_USER ?= postgres
DB_NAME ?= amz_orders
DB_PORT ?= 5432
POSTGRES_VOLUME_NAME ?= amz-orders-postgres-data
POSTGRES_VOLUME_EXTERNAL ?= false
SERVICE ?= app

.PHONY: help fresh fresh-prod dev prod dev-app prod-app build up down logs status health db-shell db-logs

help:
	@echo "Simple commands:"
	@echo "  make fresh      Fresh dev start: wipe DB volume, rebuild containers, rerun migrations"
	@echo "  make fresh-prod Fresh prod start: wipe DB volume, rebuild containers, rerun migrations, start nginx"
	@echo "  make dev        Fresh dev start: rebuild containers, recreate DB volume, rerun migrations"
	@echo "  make prod       Fresh prod start: rebuild containers, recreate DB volume, rerun migrations, start nginx"
	@echo "  make dev-app    Rebuild and restart only the Go app container, keep DB and volume intact"
	@echo "  make prod-app   Rebuild and restart only the Go app container in prod, keep DB and volume intact"
	@echo "  make up         Start services without deleting the DB volume"
	@echo "  make down       Stop services and remove the DB volume"
	@echo "  make logs       Tail logs for SERVICE=app by default"
	@echo "  make status     Show container status"
	@echo "  make health     Check backend health endpoint"
	@echo "  make db-shell   Open psql in the running Postgres container"
	@echo "  make db-logs    Tail Postgres logs"
	@echo ""
	@echo "Overrides:"
	@echo "  ENV_FILE=.env.docker"
	@echo "  APP_PORT=8080"
	@echo "  DB_USER=postgres DB_NAME=amz_orders DB_PORT=5432"
	@echo "  POSTGRES_VOLUME_NAME=amz-orders-postgres-data"

dev:
	@echo "Starting fresh development stack..."
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL=false \
	$(COMPOSE) --env-file "$(ENV_FILE)" down -v
	-docker volume rm -f "$(POSTGRES_VOLUME_NAME)"
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL=false \
	$(COMPOSE) --env-file "$(ENV_FILE)" up -d --build postgres app
	@echo "Development stack started fresh."

fresh: dev

dev-go-only:
	@echo "Restarting only the Go app container for development..."
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL="$(POSTGRES_VOLUME_EXTERNAL)" \
	$(COMPOSE) --env-file "$(ENV_FILE)" stop app
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL="$(POSTGRES_VOLUME_EXTERNAL)" \
	$(COMPOSE) --env-file "$(ENV_FILE)" up -d --build app
	@echo "Development app container restarted. Database was left untouched."

build:
	$(COMPOSE) --env-file "$(ENV_FILE)" build app

up:
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL="$(POSTGRES_VOLUME_EXTERNAL)" \
	$(COMPOSE) --env-file "$(ENV_FILE)" up -d --build postgres app

prod:
	@echo "Starting fresh production stack..."
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL=false \
	$(COMPOSE) --env-file "$(ENV_FILE)" down -v
	-docker volume rm -f "$(POSTGRES_VOLUME_NAME)"
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL=false \
	$(COMPOSE) --env-file "$(ENV_FILE)" --profile production up -d --build
	@echo "Production stack started fresh."

fresh-prod: prod

prod-app:
	@echo "Restarting only the Go app container for production..."
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL="$(POSTGRES_VOLUME_EXTERNAL)" \
	$(COMPOSE) --env-file "$(ENV_FILE)" --profile production stop app
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL="$(POSTGRES_VOLUME_EXTERNAL)" \
	$(COMPOSE) --env-file "$(ENV_FILE)" --profile production up -d --build app
	@echo "Production app container restarted. Database and pgdata volume were left untouched."

down:
	$(COMPOSE) --env-file "$(ENV_FILE)" down -v

logs:
	$(COMPOSE) --env-file "$(ENV_FILE)" logs -f "$(SERVICE)"

status:
	$(COMPOSE) --env-file "$(ENV_FILE)" ps

health:
	curl -fsS "http://localhost:$(APP_PORT)/health"

db-shell:
	POSTGRES_VOLUME_NAME="$(POSTGRES_VOLUME_NAME)" POSTGRES_VOLUME_EXTERNAL="$(POSTGRES_VOLUME_EXTERNAL)" \
	$(COMPOSE) --env-file "$(ENV_FILE)" exec postgres psql -U "$(DB_USER)" -d "$(DB_NAME)"

db-logs:
	$(COMPOSE) --env-file "$(ENV_FILE)" logs -f postgres



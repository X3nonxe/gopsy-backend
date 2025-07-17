# Makefile - Fixed version
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Variabel
MIGRATION_DIR := migrations
DOCKER_COMPOSE := docker-compose
APP_ENTRYPOINT := cmd/main.go
IS_IN_PROGRESS = "sedang diproses ..."

# Definisikan URL database untuk migrasi
# Menggunakan sslmode=disable untuk development local
DB_URL := postgresql://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable

.PHONY: all
all: help

## help: Menampilkan pesan bantuan ini
.PHONY: help
help:
	@echo "Penggunaan: \n"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## setup: Menjalankan container Docker (app & db) di background
.PHONY: setup
setup:
	@echo "make setup ${IS_IN_PROGRESS}"
	@$(DOCKER_COMPOSE) up -d --build
	@echo "Menunggu database siap..."
	@sleep 8

## down: Menghentikan dan menghapus container Docker
.PHONY: down
down: 
	@echo "make down ${IS_IN_PROGRESS}"
	@$(DOCKER_COMPOSE) down -v --remove-orphans

## run: Menjalankan aplikasi secara lokal (membutuhkan Go terinstall)
.PHONY: run
run:
	@echo "Menjalankan aplikasi di port ${APP_PORT}..."
	@go run ${APP_ENTRYPOINT}

# seed: Menjalankan seeder untuk mengisi data awal
.PHONY: seed
seed:
	@echo "Menjalankan seeder untuk mengisi data awal..."
	@go run cmd/seeder/main.go

## install-tools: Menginstall tools yang dibutuhkan seperti migrate dan mockgen
.PHONY: install-tools
install-tools:
	@echo "Menginstall tools development..."
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@go install github.com/golang/mock/mockgen@v1.6.0

## mod: Membersihkan dan mengunduh semua dependensi Go
.PHONY: mod
mod:
	@echo "make mod ${IS_IN_PROGRESS}"
	@go mod tidy
	@go mod vendor

## create-migration: Membuat file migrasi SQL baru
# Contoh penggunaan: make create-migration name=add_schedules_table
.PHONY: create-migration
create-migration:
	@echo "Membuat file migrasi baru bernama '${name}'..."
	@migrate create -ext sql -dir ${MIGRATION_DIR} -seq ${name}

## migrate-up: Menjalankan migrasi database ke versi terbaru
.PHONY: migrate-up
migrate-up:
	@echo "Menjalankan migrasi up..."
	@echo "Database URL: ${DB_URL}"
	@migrate -path ${MIGRATION_DIR} -database "${DB_URL}" up

## migrate-down: Me-revert migrasi database terakhir
.PHONY: migrate-down
migrate-down: 
	@echo "Menjalankan migrasi down..."
	@migrate -path ${MIGRATION_DIR} -database "${DB_URL}" down

## migrate-force: Memaksa set versi migrasi (gunakan dengan hati-hati)
# Contoh: make migrate-force version=1
.PHONY: migrate-force
migrate-force:
	@echo "Memaksa set versi migrasi ke ${version}..."
	@migrate -path ${MIGRATION_DIR} -database "${DB_URL}" force ${version}

## migrate-version: Menampilkan versi migrasi saat ini
.PHONY: migrate-version
migrate-version:
	@echo "Checking migration version..."
	@migrate -path ${MIGRATION_DIR} -database "${DB_URL}" version

## db-reset: Reset database (down all migrations then up)
.PHONY: db-reset
db-reset:
	@echo "Resetting database..."
	@make migrate-down
	@make migrate-up

## mock: Membuat mock untuk semua interface domain
.PHONY: mock
mock:
	@echo "Membuat mock untuk repository dan usecase..."
	@mockgen -source=internal/domain/user.go -destination=internal/mocks/user_mocks.go -package=mocks
	@mockgen -source=internal/domain/availability.go -destination=internal/mocks/availability_mocks.go -package=mocks


## test-unit: Menjalankan unit test untuk usecase
.PHONY: test-unit
test-unit:
	@echo "make test-unit ${IS_IN_PROGRESS}"
	@go clean -testcache
	@go test ./internal/usecase/... -v -cover -coverprofile=coverage-unit.out

## test-integration: Menjalankan integration test untuk repository
.PHONY: test-integration
test-integration:
	@echo "make test-integration ${IS_IN_PROGRESS}"
	@go clean -testcache
	@go test ./internal/repository/... -v -tags=integration -cover -coverprofile=coverage-integration.out

# test-k6: Menjalankan test k6 untuk API
.PHONY: test-k6
test-k6:
	@echo "Menjalankan k6 API/Load Tests..."
	@echo "Pastikan aplikasi sedang berjalan (via 'make run' atau 'make setup')."
	@k6 run tests/k6/availability_test.js

## test: Menjalankan semua test (unit & integration)
# Ini akan setup database, menjalankan migrasi, menjalankan test, dan membersihkan kembali
.PHONY: test-all
test-all:
	@make test-unit
	@make setup
	@make migrate-up
	@make test-integration
	@make test-k6
	@make down

## cover: Menampilkan laporan coverage dari semua test
.PHONY: cover
cover:
	@echo "Laporan Test Coverage:"
	@go tool cover -func=coverage-unit.out
	@go tool cover -func=coverage-integration.out

## docker-logs: Menampilkan logs dari container
.PHONY: docker-logs
docker-logs:
	@$(DOCKER_COMPOSE) logs -f

## docker-ps: Menampilkan status container
.PHONY: docker-ps
docker-ps:
	@$(DOCKER_COMPOSE) ps

## clean: Membersihkan file coverage dan cache
.PHONY: clean
clean:
	@echo "Membersihkan file coverage dan cache..."
	@rm -f coverage-*.out
	@go clean -testcache
	@go clean -cache
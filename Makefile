-include .env

ADDR=localhost:8085

DB_SERVICE=db
TEST_DB_SERVICE=test_db

.PHONY: run-debug build migrate db-up db-recreate test-db-up test-db-down


run-debug: 
	migrate -database $(DB_CONNECTION_STRING) -path db/migrations up
	wgo -file=.templ -file=.go -xfile=_templ.go templ generate :: go fmt ./... :: go run . -address=$(ADDR)

migrate:
	migrate -database $(DB_CONNECTION_STRING) -path db/migrations up

db-up:
	docker compose up -d $(DB_SERVICE)
	@echo "Postgres up and running at port: $(POSTGRES_PORT)"

db-down:
	docker compose stop $(DB_SERVICE)
	@echo "Postgres container stopped"

db-restart:
	docker compose restart $(DB_SERVICE)
	@echo "Postgres restarted"

db-remove:
	docker compose rm -s $(DB_SERVICE)
	@echo "Postgres container removed"

db-remove-volume:
	docker compose rm -s $(DB_SERVICE)
	docker volume rm $(DOCKER_POSTGRES_VOLUME)
	@echo "Postgres container removed with volume"

db-recreate: 
	docker compose rm -sf $(DB_SERVICE)
	docker volume rm $(DOCKER_POSTGRES_VOLUME)
	docker compose up -d --wait --wait-timeout 60  $(DB_SERVICE)
	migrate -database $(DB_CONNECTION_STRING) -path db/migrations up

db-shell:
	docker compose exec $(DB_SERVICE) sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

test-db-up:
	docker compose up -d $(TEST_DB_SERVICE)
	@echo "Test postgres up and running at port: $(POSTGRES_TEST_PORT)"

test-db-down:
	docker compose rm -s $(TEST_DB_SERVICE)
	@echo "Test postgres container removed"

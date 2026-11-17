SERVICE ?= wallet
DB_URL  ?= postgres://payments:payments@localhost:5432/$(SERVICE)?sslmode=disable
MIGRATE := migrate -path db/$(SERVICE)/migrations -database "$(DB_URL)"

.PHONY: migrate-up migrate-down migrate-version migrate-new

migrate-up:
	@$(MIGRATE) up

migrate-down:
	@$(MIGRATE) down 1

migrate-force:
	@$(MIGRATE) force $(version)

migrate-version:
	@$(MIGRATE) version

migrate-new:
	migrate create -ext sql -dir db/$(SERVICE)/migrations -seq $(name)

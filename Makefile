.PHONY: up down shell redis-cli test fmt vet ps

up:
	docker compose up -d

down:
	docker compose down

shell:
	docker compose exec app sh

redis-cli:
	docker compose exec redis redis-cli

test:
	docker compose exec app go test ./...

fmt:
	docker compose exec app go fmt ./...

vet:
	docker compose exec app go vet ./...

ps:
	docker compose ps

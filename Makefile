.PHONY: up down reset test logs

up:
	docker compose up -d --build

down:
	docker compose down

reset:
	docker compose down -v && docker compose up -d --build

test:
	go test ./...

logs:
	docker compose logs -f app

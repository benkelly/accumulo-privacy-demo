.PHONY: up down logs clean

# Builds the images, starts the stack and waits until Accumulo is healthy.
up:
	docker compose up -d --build --wait --wait-timeout 300

down:
	docker compose down

logs:
	docker compose logs -f

# Also deletes all stored data.
clean:
	docker compose down --volumes

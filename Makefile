.PHONY: up down seed unit logs clean

# Builds the images, starts the stack, waits until it is healthy, then seeds the data.
up:
	docker compose up -d --build --wait --wait-timeout 300
	$(MAKE) seed

# Writes the 20 fake patients again. This also puts back any deleted patient.
# Needs the stack to be up. --no-deps stops Compose from recreating the running services.
seed:
	docker compose run --rm --no-deps seed

# Unit tests, run in Docker so you do not need Java or Maven installed.
unit:
	docker run --rm -v "$(CURDIR)/gateway:/src" -v accumulo-privacy-demo-m2:/root/.m2 -w /src \
		maven:3.9.16-eclipse-temurin-17 mvn -B -q test

down:
	docker compose down

logs:
	docker compose logs -f

# Also deletes all stored data.
clean:
	docker compose down --volumes

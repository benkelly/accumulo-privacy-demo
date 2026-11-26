.PHONY: up down seed unit logs clean

# The build images from the Dockerfiles, so Dependabot updates one place.
MAVEN_IMAGE := $(shell sed -n 's/^FROM \(maven:[^ ]*\).*/\1/p' gateway/Dockerfile)
GO_IMAGE := $(shell sed -n 's/^FROM \(golang:[^ ]*\).*/\1/p' api/Dockerfile)

# Builds the images, starts the stack, waits until it is healthy, then seeds the data.
up:
	docker compose up -d --build --wait --wait-timeout 300
	$(MAKE) seed

# Writes the 20 fake patients again. This also puts back any deleted patient.
# Needs the stack to be up. --no-deps stops Compose from recreating the running services.
seed:
	docker compose run --rm --no-deps seed

# Unit tests, run in Docker so you do not need Java, Maven or Go installed.
unit:
	docker run --rm -v "$(CURDIR)/api:/src" -w /src $(GO_IMAGE) go test ./...
	docker run --rm -v "$(CURDIR)/gateway:/src" -v accumulo-privacy-demo-m2:/root/.m2 -w /src \
		$(MAVEN_IMAGE) mvn -B -q test

down:
	docker compose down

logs:
	docker compose logs -f

# Also deletes all stored data.
clean:
	docker compose down --volumes

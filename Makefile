.PHONY: up down seed lint unit test logs clean

# The build images from the Dockerfiles, so Dependabot updates one place.
MAVEN_IMAGE := $(shell sed -n 's/^FROM \(maven:[^ ]*\).*/\1/p' gateway/Dockerfile)
GO_IMAGE := $(shell sed -n 's/^FROM \(golang:[^ ]*\).*/\1/p' api/Dockerfile)
NODE_IMAGE := $(shell sed -n 's/^FROM \(node:[^ ]*\).*/\1/p' web/Dockerfile)
M2 := -v accumulo-privacy-demo-m2:/root/.m2

# Builds the images, starts the stack, waits until it is healthy, then seeds the data.
up:
	docker compose up -d --build --wait --wait-timeout 300
	$(MAKE) seed

# Writes the 20 fake patients again. This also puts back any deleted patient.
# Needs the stack to be up. --no-deps stops Compose from recreating the running services.
seed:
	docker compose run --rm --no-deps seed

# gofmt and go vet, Checkstyle, and ESLint, all in Docker.
# The anonymous volume keeps Linux node_modules out of your web/ folder.
lint:
	docker run --rm -v "$(CURDIR)/api:/src" -w /src $(GO_IMAGE) \
		sh -c 'f=$$(gofmt -l .); [ -z "$$f" ] || { echo "gofmt: $$f"; exit 1; }; go vet -tags e2e ./...'
	docker run --rm -v "$(CURDIR)/gateway:/src" $(M2) -w /src $(MAVEN_IMAGE) mvn -B -q checkstyle:check
	docker run --rm -v "$(CURDIR)/web:/src" -v /src/node_modules -w /src $(NODE_IMAGE) \
		sh -c 'npm ci --no-audit --no-fund && npm run lint'

# Unit tests, run in Docker so you do not need Java, Maven or Go installed.
# A failed gateway test runs once more: the garbage collector test depends on timing.
unit:
	docker run --rm -v "$(CURDIR)/api:/src" -w /src $(GO_IMAGE) go test ./...
	docker run --rm -v "$(CURDIR)/gateway:/src" $(M2) -w /src $(MAVEN_IMAGE) \
		mvn -B -q test -Dsurefire.rerunFailingTestsCount=1

# End to end: starts and seeds the stack, runs api/e2e_test.go against it, then seeds again.
test: up
	docker run --rm --network accumulo-privacy-demo_default -v "$(CURDIR)/api:/src" -w /src \
		-e API_URL=http://api:8080 -e WEB_URL=http://web:8080 $(GO_IMAGE) \
		go test -tags e2e -count=1 -run E2E -v ./...
	$(MAKE) seed

down:
	docker compose down

logs:
	docker compose logs -f

# Also deletes all stored data.
clean:
	docker compose down --volumes

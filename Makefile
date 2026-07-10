APP_NAME := auren-transfer-agent
CMD_PATH := ./cmd/agent
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)
VERSION ?= v1.9.0
IMAGE ?= auren-transfer-agent:$(VERSION)

.PHONY: help tidy fmt test build run serve version clean docker-build compose-up deb apt-repo apt-publish release release-dry-run

help:
	@echo "Auren Transfer Agent"
	@echo ""
	@echo "Targets:"
	@echo "  make tidy           - tidy Go modules"
	@echo "  make fmt            - format Go files"
	@echo "  make test           - run tests"
	@echo "  make build          - build binary"
	@echo "  make run            - run the agent with default configuration"
	@echo "  make serve          - run HTTP server locally"
	@echo "  make version        - print version metadata"
	@echo "  make docker-build   - build Docker image"
	@echo "  make compose-up     - start Docker Compose stack"
	@echo "  make deb            - build Debian package"
	@echo "  make apt-repo       - build local APT repository"
	@echo "  make apt-publish    - publish APT repository to S3 using S3_URI"
	@echo "  make release        - create release archive"
	@echo "  make release-dry-run - validate release pipeline"
	@echo "  make clean          - remove build artifacts"

tidy:
	go mod tidy

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

test:
	go test ./...

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_PATH) $(CMD_PATH)

run:
	go run $(CMD_PATH)

serve:
	AUREN_SERVER_ENABLED=true go run $(CMD_PATH) serve --config ./configs/agent.yaml

version:
	go run $(CMD_PATH) --version

docker-build:
	docker build -f docker/Dockerfile -t $(IMAGE) .

compose-up:
	docker compose -f docker/docker-compose.yml up --build

deb: build
	./scripts/build-deb.sh $(VERSION)

apt-repo: deb
	./scripts/build-apt-repo.sh

apt-publish: apt-repo
	./scripts/publish-apt-s3.sh

release:
	./scripts/release.sh $(VERSION)

release-dry-run:
	./scripts/release.sh $(VERSION) --dry-run

clean:
	rm -rf $(BIN_DIR) dist

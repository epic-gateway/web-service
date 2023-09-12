REPO ?= quay.io/epic-gateway
PREFIX ?= web-service
SUFFIX ?= ${USER}-dev

TAG ?= ${REPO}/${PREFIX}:${SUFFIX}

##@ Default Goal
.PHONY: help
help: ## Display this help
	@echo "Usage:"
	@echo "  make <goal> [VAR=value ...]"
	@echo ""
	@echo "Variables"
	@echo "  REPO   The registry part of the Docker tag"
	@echo "  PREFIX Docker tag prefix (after the registry, before the suffix)"
	@echo "  SUFFIX Docker tag suffix (the part after ':')"
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  %-15s %s\n", $$1, $$2 } \
		/^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development Goals

.PHONY: test
test: ## Run some code quality checks
	go vet ./...
	go test -race -short ./...

run: ## Run the service using "go run" (KUBECONFIG needs to be set)
	go run ./main.go

docker-build:	## Build the Docker image
	docker build --tag=${TAG} ${DOCKER_BUILD_OPTIONS} .

docker-push:	docker-build ## Push the image to the registry
	docker push ${TAG}

.PHONY: manifest
manifest: deploy/web-service.yaml

deploy/web-service.yaml: config/web-service.yaml
	sed "s quay.io/epic-gateway/web-service:unknown ${TAG} " < $^ > $@
	cp deploy/web-service.yaml deploy/web-service-${SUFFIX}.yaml

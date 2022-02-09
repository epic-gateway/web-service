REPO ?= registry.gitlab.com/acnodal/epic
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

.PHONY: check
check: ## Run some code quality checks
	go vet ./...
	go test -race -short ./...

run: ## Run the service using "go run" (KUBECONFIG needs to be set)
	go run ./main.go

image:	## Build the Docker image (GITLAB_AUTHN needs to be set)
	docker build --build-arg=GITLAB_USER --build-arg=GITLAB_PASSWORD --tag=${TAG} ${DOCKER_BUILD_OPTIONS} .

install:	image ## Push the image to the registry
	docker push ${TAG}

.PHONY: manifest
manifest: deploy/web-service.yaml

deploy/web-service.yaml: config/web-service.yaml
	sed "s registry.gitlab.com/acnodal/epic/web-service:unknown ${TAG} " < $^ > $@
	cp deploy/web-service.yaml deploy/web-service-${SUFFIX}.yaml

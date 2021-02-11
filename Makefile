REPO ?= registry.gitlab.com/acnodal
PREFIX ?= egw-web-service
SUFFIX ?= ${USER}-dev

TAG ?= ${REPO}/${PREFIX}:${SUFFIX}
DOCKERFILE ?= build/package/Dockerfile

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
	golint -set_exit_status ./...
	go test -race -short ./...

run: ## Run the service using "go run" (KUBECONFIG needs to be set)
	go run ./main.go

image:	## Build the Docker image (GITLAB_AUTHN needs to be set)
	docker build --build-arg=GITLAB_AUTHN --file=${DOCKERFILE} --tag=${TAG} ${DOCKER_BUILD_OPTIONS} .

install:	image ## Push the image to the registry
	docker push ${TAG}

.PHONY: manifest
manifest: deploy/egw-web-service.yaml

deploy/egw-web-service.yaml: config/egw-web-service.yaml
	sed "s registry.gitlab.com/acnodal/egw-web-service:unknown ${TAG} " < $^ > $@

PREFIX = egw
SUFFIX = ${USER}-dev
SHELL:=/bin/bash

TAG=${PREFIX}/web-service:${SUFFIX}
DOCKERFILE=build/package/Dockerfile

ifndef GITLAB_TOKEN
$(error GITLAB_TOKEN not set. It must contain a gitlab Personal Access Token with repo read access)
endif

##@ Default Goal
.PHONY: help
help: ## Display this help
	@echo "Usage:\n  make <goal> [VAR=value ...]"
	@echo "\nVariables"
	@echo "  PREFIX Docker tag prefix (useful to set the docker registry)"
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

run: ## Run the service using "go run"
	go run ./main.go

image: ## Build the Docker image
	@docker build --build-arg=GITLAB_TOKEN --file=${DOCKERFILE} --tag=${TAG} .

runimage: image ## Run the service using "docker run"
	docker run --rm --publish 8080:8080 ${TAG}

install:	image ## Push the image to the repo
	docker push ${TAG}

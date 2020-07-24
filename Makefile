.DEFAULT_GOAL:=help
SHELL:=/bin/bash

TAG=registry.gitlab.com/acnodal/egw-web-service/web-service:${USER}-dev
DOCKERFILE=build/package/Dockerfile

##@ Development

run: ## Run the service using "go run"
	go run ./cmd/egw-ws

image: ## Build the Docker image
	docker build --file=${DOCKERFILE} --tag=${TAG} .

runimage: image ## Run the service using "docker run"
	docker run --rm --publish 8080:8080 ${TAG}

push:	image ## Push the image to the repo
	docker push ${TAG}

.PHONY: help
help: ## Display this help
	@echo -e "Usage:\n  make <target>"
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  %-15s %s\n", $$1, $$2 } \
		/^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

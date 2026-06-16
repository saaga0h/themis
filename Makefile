IMAGE := themis:dev
MAX_TURNS ?= 600
PROVIDER ?= gitea
BINARY     := themis
CMD        := ./cmd/themis
BUILD_DIR  := bin

PODMAN_RUN := podman run -i \
	--userns=keep-id \
	--entrypoint claude \
	-v $(PWD):/home/agent/workspace \
	-v $(HOME)/.claude:/home/agent/.claude \
	--env-file .env \
	-w /home/agent/workspace \
	--memory=12g \
	$(IMAGE) \
	--verbose \
	--dangerously-skip-permissions \
	--max-turns $(MAX_TURNS)

.PHONY: run-factory dry-run run-issue shell build-image build test lint

run-factory: ## Run the autonomous factory loop
	echo '/factory --provider $(PROVIDER)' | $(PODMAN_RUN)

dry-run: ## Preview which issues would be processed
	echo '/factory --provider $(PROVIDER) --dry-run' | $(PODMAN_RUN)

run-issue: ## Run a single issue: make run-issue ISSUE=3
	echo '/issue $(ISSUE) --provider $(PROVIDER)' | $(PODMAN_RUN)

shell: ## Open an interactive shell inside the factory container
	podman run -it --userns=keep-id --entrypoint /bin/bash \
		-v $(PWD):/home/agent/workspace \
		-v $(HOME)/.claude:/home/agent/.claude \
		--env-file .env -w /home/agent/workspace \
		--memory=16g $(IMAGE)

build-image: ## Build the factory container image
	podman build -t $(IMAGE) --memory=16g .

build: ## Build the themis binary
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

run: build ## Build and run
	$(BUILD_DIR)/$(BINARY)

test: ## Run Go tests
	go test ./...

lint: ## Run Go vet
	go vet ./...

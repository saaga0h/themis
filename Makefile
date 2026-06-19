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

.PHONY: run-factory dry-run run-issue run-v2-issue factory-cc shell build-image build test lint

factory-cc: ## Materialize the curated .claude/ the factory container overlays (from factory/manifest.txt)
	@rm -rf .themis/factory-cc/.claude
	@mkdir -p .themis/factory-cc/.claude/skills .themis/factory-cc/.claude/agents .themis/factory-cc/.claude/commands
	@printf '{\n  "includeCoAuthoredBy": false,\n  "disableBundledSkills": true\n}\n' > .themis/factory-cc/.claude/settings.json
	@grep -vE '^[[:space:]]*(#|$$)' factory/manifest.txt | while read -r kind name; do \
		case "$$kind" in \
			skill)   cp -R "skills/$$name" ".themis/factory-cc/.claude/skills/$$name" ;; \
			agent)   cp "agents/$$name.md" ".themis/factory-cc/.claude/agents/$$name.md" ;; \
			command) cp "commands/$$name.md" ".themis/factory-cc/.claude/commands/$$name.md" ;; \
			*) echo "factory-cc: unknown manifest kind '$$kind'" >&2; exit 1 ;; \
		esac; \
	done
	@echo "factory-cc: curated .claude/ ready per factory/manifest.txt"

run-factory: ## Run the autonomous factory loop
	echo '/factory --provider $(PROVIDER)' | $(PODMAN_RUN)

dry-run: ## Preview which issues would be processed
	echo '/factory --provider $(PROVIDER) --dry-run' | $(PODMAN_RUN)

run-issue: ## Run a single issue: make run-issue ISSUE=3
	echo '/issue $(ISSUE) --provider $(PROVIDER)' | $(PODMAN_RUN)

run-v2-issue: factory-cc ## Run v2.0 binary against a single issue
	podman run -i --userns=keep-id \
		--entrypoint bash \
		-v $(PWD):/home/agent/workspace \
		-v $(PWD)/.themis/factory-cc/.claude:/home/agent/workspace/.claude \
		--env-file .env \
		-w /home/agent/workspace \
		--memory=12g \
		$(IMAGE) \
		-c 'go build -o /tmp/themis ./cmd/themis/ && /tmp/themis issue $(ISSUE) --provider gitea'

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

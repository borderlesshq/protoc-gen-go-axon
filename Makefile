GOPATH:=$(shell go env GOPATH)

GO_SOURCES := $(wildcard *.go)
GO_SOURCES += $(shell find . -type f -name "*.go")

GOFMT ?= gofmt -s

ifeq ($(filter $(TAGS_SPLIT),bindata),bindata)
	GO_SOURCES += $(BINDATA_DEST)
endif

GO_SOURCES_OWN := $(filter-out outlet/%, $(GO_SOURCES))





vet:
	go vet -v ./...

fmt:
	gofmt -w .

fmt-check:
	@diff=$$($(GOFMT) -d $(GO_SOURCES_OWN)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

alignment:
	go run golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment -fix ./models > /dev/null 2>&1 || :


# ============================================================================
# Protocol Buffers Generation
# ============================================================================
# Proto configuration
# Proto configuration
PROTO_SRC_BASE := ${PWD}/protos
PROTO_DST_BASE := ${PWD}/contracts
# Automatically discover all proto directories under protos/
# This will find: protos/accounts, protos/cards, etc.
PROTO_ENTITIES := $(shell find $(PROTO_SRC_BASE) -mindepth 1 -maxdepth 1 -type d -exec basename {} \;)
GO_MODULE := $(shell go list -m | head -n 1)

.PHONY: clean-proto clean-proto-generated clean-proto-plugins install-proto-plugins proto protos list-protos

# Clean all generated proto files
clean-proto-generated:
	@echo "🧹 Cleaning generated proto files..."
	@find ${PROTO_DST_BASE} -name "*_axon.pb.go" -type f -delete 2>/dev/null || true
	@find ${PROTO_DST_BASE} -name "*.pb.go" -type f -delete 2>/dev/null || true
	@echo "✓ Generated files cleaned"

# Clean installed protoc plugins
clean-proto-plugins:
	@echo "🧹 Cleaning installed protoc plugins..."
	@rm -f $(GOPATH)/bin/protoc-gen-go-axon
	# @rm -f $(GOPATH)/bin/protoc-gen-go
	@echo "✓ Plugins cleaned"

# Clean everything proto-related
clean-proto: clean-proto-generated clean-proto-plugins

# Install required protoc plugins
install-proto-plugins:
	@echo "📦 Installing protoc plugins..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install . -o github.com/borderlesshq/protoc-gen-go-axon@v1.0.2
	@#go build -o $(GOPATH)/bin/protoc-gen-go-axon .
	@echo "✓ Plugins installed"

# List all available proto entities
list-protos:
	@echo "Available proto entities:"
	@for entity in $(PROTO_ENTITIES); do \
		echo "  - $$entity"; \
	done

define build-proto-mappings
PROTO_MAPPINGS=""; \
for e in $(PROTO_ENTITIES); do \
	for proto in $$(find $(PROTO_SRC_BASE)/$$e -name '*.proto' -type f 2>/dev/null); do \
		proto_rel=$$(echo $$proto | sed "s|$(PROTO_SRC_BASE)/||"); \
		PROTO_MAPPINGS="$$PROTO_MAPPINGS,M$$proto_rel=$(GO_MODULE)/contracts/$$e"; \
	done; \
done; \
PROTO_MAPPINGS=$$(echo "$$PROTO_MAPPINGS" | sed 's/^,//')
endef

# Generate proto for a SPECIFIC entity
proto: install-proto-plugins
	@if [ -z "$(ENTITY)" ]; then \
		echo "❌ Error: ENTITY not specified"; \
		echo "Usage: make proto ENTITY=accounts"; \
		echo ""; \
		echo "Available entities:"; \
		for entity in $(PROTO_ENTITIES); do \
			echo "  - $$entity"; \
		done; \
		exit 1; \
	fi
	@echo "🔄 Generating proto for entity: $(ENTITY)"
	@echo "   Module: $(GO_MODULE)"
	@PROTO_SRC_DIR=$(PROTO_SRC_BASE)/$(ENTITY); \
	PROTO_DST_DIR=$(PROTO_DST_BASE)/$(ENTITY); \
	if [ ! -d "$$PROTO_SRC_DIR" ]; then \
		echo "❌ Error: Directory $$PROTO_SRC_DIR does not exist"; \
		exit 1; \
	fi; \
	echo "   Source: $$PROTO_SRC_DIR"; \
	echo "   Output: $$PROTO_DST_DIR"; \
	mkdir -p $$PROTO_DST_DIR; \
	@$(build-proto-mappings); \
	if [ -n "$$(find $$PROTO_SRC_DIR -name '*.proto' -type f 2>/dev/null)" ]; then \
		protoc \
			-I=$(PROTO_SRC_BASE) \
			--go_out=$(PROTO_DST_BASE) \
			--go_opt=paths=source_relative \
			--go_opt=$$PROTO_MAPPINGS \
			--go-axon_out=$(PROTO_DST_BASE) \
			--go-axon_opt=paths=source_relative \
			--go-axon_opt=$$PROTO_MAPPINGS \
			$$PROTO_SRC_DIR/*.proto && \
		echo "✅ Generated $(ENTITY) successfully" || \
		(echo "❌ Failed to generate $(ENTITY)"; exit 1); \
	else \
		echo "❌ No .proto files found in $$PROTO_SRC_DIR"; \
		exit 1; \
	fi

# Generate protos for ALL entities
protos: clean-proto-generated install-proto-plugins
	@echo "🔄 Generating protos for all entities..."
	@echo "   Module: $(GO_MODULE)"
	@echo "   Building import mappings..."
	@$(build-proto-mappings); \
	for entity in $(PROTO_ENTITIES); do \
		echo ""; \
		echo "📝 Processing: $$entity"; \
		PROTO_SRC_DIR=$(PROTO_SRC_BASE)/$$entity; \
		PROTO_DST_DIR=$(PROTO_DST_BASE)/$$entity; \
		echo "   Source: $$PROTO_SRC_DIR"; \
		echo "   Output: $$PROTO_DST_DIR"; \
		mkdir -p $$PROTO_DST_DIR; \
		if [ -n "$$(find $$PROTO_SRC_DIR -name '*.proto' -type f 2>/dev/null)" ]; then \
			protoc \
				-I=$(PROTO_SRC_BASE) \
				--go_out=$(PROTO_DST_BASE) \
				--go_opt=paths=source_relative \
				--go_opt=$$PROTO_MAPPINGS \
				--go-axon_out=$(PROTO_DST_BASE) \
				--go-axon_opt=paths=source_relative \
				--go-axon_opt=$$PROTO_MAPPINGS \
				$$PROTO_SRC_DIR/*.proto && \
			echo "   ✓ Generated $$entity successfully" || \
			echo "   ✗ Failed to generate $$entity"; \
		else \
			echo "   ⚠ No .proto files found in $$PROTO_SRC_DIR"; \
		fi; \
	done
	@echo ""
	@echo "✅ All protos generated"

# Dynamically create proto-<entity> targets for each discovered entity
# This allows: make proto-accounts, make proto-cards, etc.
.PHONY: $(addprefix proto-,$(PROTO_ENTITIES))

$(addprefix proto-,$(PROTO_ENTITIES)): proto-%:
	@$(MAKE) proto ENTITY=$*

# ============================================================================
# Watch mode (optional - requires fswatch or inotifywait)
# ============================================================================

.PHONY: watch-protos
watch-protos:
	@echo "👀 Watching for proto file changes..."
	@echo "Press Ctrl+C to stop"
	@if command -v fswatch > /dev/null; then \
		fswatch -o $(PROTO_SRC_BASE) | while read; do \
			echo ""; \
			echo "📝 Changes detected, regenerating..."; \
			$(MAKE) protos; \
		done; \
	elif command -v inotifywait > /dev/null; then \
		while inotifywait -r -e modify,create,delete $(PROTO_SRC_BASE); do \
			echo ""; \
			echo "📝 Changes detected, regenerating..."; \
			$(MAKE) protos; \
		done; \
	else \
		echo "❌ Neither fswatch nor inotifywait found"; \
		echo "Install one of them to use watch mode:"; \
		echo "  macOS: brew install fswatch"; \
		echo "  Linux: apt-get install inotify-tools"; \
		exit 1; \
	fi

# ============================================================================
# Help target
# ============================================================================

.PHONY: help-proto
help-proto:
	@echo "Protocol Buffers Makefile Targets"
	@echo "=================================="
	@echo ""
	@echo "Main targets:"
	@echo "  make protos              - Generate protos for ALL entities"
	@echo "  make proto ENTITY=<name> - Generate proto for specific entity"
	@echo "  make proto-<entity>      - Shortcut for specific entity"
	@echo ""
	@echo "Cleanup targets:"
	@echo "  make clean-proto         - Clean all generated files and plugins"
	@echo "  make clean-proto-generated - Clean only generated .pb.go files"
	@echo "  make clean-proto-plugins - Clean only installed plugins"
	@echo ""
	@echo "Utility targets:"
	@echo "  make list-protos         - List all available proto entities"
	@echo "  make install-proto-plugins - Install protoc plugins"
	@echo "  make watch-protos        - Watch for changes and auto-regenerate"
	@echo ""
	@echo "Available entities:"
	@for entity in $(PROTO_ENTITIES); do \
		echo "  - $$entity (make proto-$$entity)"; \
	done
	@echo ""
	@echo "Examples:"
	@echo "  make protos                    # Generate all"
	@echo "  make proto ENTITY=accounts     # Generate accounts only"
	@echo "  make proto-accounts            # Same as above"
	@echo "  make proto-cards               # Generate cards only"
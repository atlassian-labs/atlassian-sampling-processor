include Makefile.common

FIND_MOD_OPT           := -type f -name "go.mod"
TO_MOD_DIR             := dirname '{}' \; | sort | grep -E '^\.\/'
ALL_COMPONENTS         := $(sort $(shell find ./pkg ./internal $(FIND_MOD_OPT) -exec $(TO_MOD_DIR)))
ALL_TOOLS_PACKAGES     := $(shell grep -E '(^|\s)_\s+\".*\"$$' < $(TOOLS_SRC_DIR)/tools.go | tr -d '"' | awk '{print $$2;}')
ALL_TOOLS_COMMAND      := $(sort $(addprefix $(TOOLS_DIR)/,$(notdir $(ALL_TOOLS_PACKAGES))))
UPSTREAM_PROJECT_DIR   ?= $(PROJECT_ROOT_DIR)/../observability-sidecar

.PHONY: tools
tools: $(ALL_TOOLS_COMMAND)

$(ALL_TOOLS_COMMAND): $(TOOLS_DIR) $(TOOLS_SRC_DIR)/go.mod
	$(GOCMD) build \
		-C $(TOOLS_SRC_DIR) \
		-o $(TOOLS_DIR)/$(notdir $@) \
		$(filter %/$(notdir $@),$(ALL_TOOLS_PACKAGES))

$(TOOLS_DIR):
	mkdir $(TOOLS_DIR)

.PHONY: $(ALL_COMPONENTS)
$(ALL_COMPONENTS):
	@$(MAKE) -C $@ info $(TARGET)

.PHONY: for-all-target
for-all-target: $(ALL_COMPONENTS)

.PHONY: all
all: all-porto all-tidy all-test

.PHONY: all-porto
all-porto:
	@$(MAKE) for-all-target TARGET=porto

.PHONY: all-test
all-test:
	@$(MAKE) for-all-target TARGET=test

.PHONY: all-tidy
all-tidy:
	@$(MAKE) for-all-target TARGET=tidy

## Must have observability-sidecar repo (Atlassian closed source) present to perform this sync
.PHONY: sync-with-upstream
sync-with-upstream:
	[ -d $(UPSTREAM_PROJECT_DIR) ] || exit 1
	echo Syncing from $(UPSTREAM_PROJECT_DIR)...
	cp -r $(UPSTREAM_PROJECT_DIR)/pkg/processor/atlassiansamplingprocessor $(PROJECT_ROOT_DIR)/pkg/processor/
	cp -r $(UPSTREAM_PROJECT_DIR)/internal/ptraceutil $(PROJECT_ROOT_DIR)/internal/
	@$(MAKE) all

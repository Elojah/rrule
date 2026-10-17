GO                 = go
GOFMT              = gofmt
# Pinned, like golangci-lint in CI. go run leaves go.mod alone.
BENCHSTAT          = $(GO) run golang.org/x/perf/cmd/benchstat@v0.0.0-20260908200009-22c9c6c9d4da

# Which benchmarks run (a regexp), how many times benchcmp runs each tree, and what it compares the working tree with.
BENCH             ?= .
BENCH_COUNT       ?= 10
BENCH_BASE        ?= main
BENCH_DIR          = .bench
BENCH_FLAGS        = -run '^$$' -bench '$(BENCH)' -benchmem

# How long fuzz runs each target, and the targets, as package:name.
FUZZ_TIME         ?= 30s
FUZZ_TARGETS       = .:FuzzParse .:FuzzParseSet .:FuzzAll

# python-dateutil, which testdata/dateutil.txt is expanded with, and six, which it needs: pinned wheels, pure Python,
# which Python imports from as they are.
PYTHON             = python3
DATEUTIL_WHEEL     = https://files.pythonhosted.org/packages/ec/57/56b9bcc3c9c6a792fcbaf139543cee77261f3651ca9da0c93f5c1221264b/python_dateutil-2.9.0.post0-py2.py3-none-any.whl
DATEUTIL_SHA256    = a8b2bc7bffae282281c8140a97d3aa9c14da0b136dfe83f850eea9a5f7470427
SIX_WHEEL          = https://files.pythonhosted.org/packages/b7/ce/149a00dd41f10bc29e5921b496af8b574d8413afcd5e30dfa0ed46c2cc5e/six-1.17.0-py2.py3-none-any.whl
SIX_SHA256         = 4721f391ed90541fddacab5acf947aa0d3dc7d27b2e1e8eda2be8970586c3274

# The packages tinygo-test covers, the examples tinygo-run and tinygo-build cover, and the board tinygo-build targets.
TINYGO             = tinygo
TINYGO_TARGET     ?= pico
TINYGO_PACKAGES    = .
EXAMPLES           = $(patsubst %/,%,$(wildcard examples/*/))
# Prints the lines under // Output: in an example's main_test.go, without their //.
EXAMPLE_OUTPUT     = awk '/\/\/ Output:/ { f = 1; next } f && /^}/ { f = 0 } f { sub(/^\t\/\/ ?/, ""); print }'

# For CI
ifneq ($(wildcard ./bin/golangci-lint),)
	GOLINT = $(CURDIR)/bin/golangci-lint
else
	GOLINT = golangci-lint
endif

V                 = 0
Q                 = $(if $(filter 1,$V),,@)
M                 = $(shell printf "\033[0;35m▶\033[0m")

.PHONY: all
all: check

# Tidy
.PHONY: tidy
tidy: ## Run go mod tidy
	$(info $(M) running go mod tidy) @
	$Q $(GO) mod tidy

# Check
.PHONY: check
check: lint test ## Run lint and tests

# Lint
.PHONY: lint
lint: ## Run golangci-lint
	$(info $(M) running $(GOLINT))
	$Q $(GOLINT) run

# Test
.PHONY: test
# GOMAXPROCS 1 interleaves goroutines differently from several cores, and -shuffle, whose seed -v prints, catches tests
# that depend on each other. Each benchmark runs once too, since go test alone compiles them but never runs them.
test: ## Run tests with race detector, shuffled, with GOMAXPROCS 1 and 4, and each benchmark once
	$(info $(M) running go test) @
	$Q $(GO) test -cover -race -shuffle=on -cpu 1,4 -v -bench . -benchtime 1x ./...

# Fuzz
.PHONY: fuzz
fuzz: ## Run each fuzz target for FUZZ_TIME (30s)
	$(info $(M) running each fuzz target for $(FUZZ_TIME)) @
	$Q for t in $(FUZZ_TARGETS); do \
		echo "$(M) $$t"; \
		$(GO) test -run '^$$' -fuzz "^$${t##*:}\$$" -fuzztime $(FUZZ_TIME) $${t%%:*} || exit 1; \
	done

# Vectors
.PHONY: vectors
# The wheels go to a directory of their own, checked, and Python imports them from there.
vectors: ## Expand random rules with python-dateutil into testdata/dateutil.txt
	$(info $(M) expanding random rules with python-dateutil) @
	$Q set -e; \
	whl=$$(mktemp -d); trap 'rm -rf "$$whl"' EXIT; \
	curl -sSfL -o "$$whl/dateutil.whl" $(DATEUTIL_WHEEL); \
	curl -sSfL -o "$$whl/six.whl" $(SIX_WHEEL); \
	echo "$(DATEUTIL_SHA256)  $$whl/dateutil.whl" | sha256sum -c --quiet; \
	echo "$(SIX_SHA256)  $$whl/six.whl" | sha256sum -c --quiet; \
	PYTHONPATH="$$whl/dateutil.whl:$$whl/six.whl" $(PYTHON) tools/vectors/gen.py > testdata/dateutil.txt

# TinyGo
.PHONY: tinygo
tinygo: tinygo-test tinygo-run tinygo-build ## Run tinygo-test, tinygo-run and tinygo-build

.PHONY: tinygo-test
tinygo-test: ## Run the tests with TinyGo
	$(info $(M) running tinygo test) @
	$Q for p in $(TINYGO_PACKAGES); do $(TINYGO) test $$p || exit 1; done

.PHONY: tinygo-run
# An example passes if it prints its Example's output block.
tinygo-run: ## Run each example with TinyGo, and diff what it prints with its Example's output
	$(info $(M) running each example with tinygo run) @
	$Q set -e; \
	out=$$(mktemp -d); trap 'rm -rf "$$out"' EXIT; \
	for e in $(EXAMPLES); do \
		echo "$(M) $$e"; \
		$(EXAMPLE_OUTPUT) $$e/main_test.go > "$$out/want"; \
		$(TINYGO) run ./$$e > "$$out/got"; \
		diff -u "$$out/want" "$$out/got"; \
	done

.PHONY: tinygo-build
# -size short prints how much flash and RAM each takes.
tinygo-build: ## Build each example with TinyGo for TINYGO_TARGET (pico), without running it
	$(info $(M) building each example for $(TINYGO_TARGET) with tinygo build) @
	$Q set -e; \
	out=$$(mktemp -d); trap 'rm -rf "$$out"' EXIT; \
	for e in $(EXAMPLES); do \
		echo "$(M) $$e"; \
		$(TINYGO) build -target=$(TINYGO_TARGET) -size short -o "$$out/main.elf" ./$$e; \
	done

# Bench
.PHONY: bench
bench: ## Run benchmarks (BENCH=regexp)
	$(info $(M) running go test -bench) @
	$Q $(GO) test $(BENCH_FLAGS) ./...

# The runs alternate between the two trees, so that drift, such as a laptop heating up, affects both alike. The results
# stay in $(BENCH_DIR), for benchstat with other flags.
.PHONY: benchcmp
benchcmp: ## Compare benchmarks of BENCH_BASE (main) and the working tree with benchstat
	$(info $(M) running benchmarks of $(BENCH_BASE) and the working tree, $(BENCH_COUNT) times each) @
	$Q set -e; \
	base=$$(mktemp -d); trap 'rm -rf "$$base"' EXIT; \
	git archive -o "$$base/src.tar" $(BENCH_BASE); tar -x -f "$$base/src.tar" -C "$$base"; \
	mkdir -p $(BENCH_DIR); rm -f $(BENCH_DIR)/base.txt $(BENCH_DIR)/worktree.txt; \
	for i in $$(seq $(BENCH_COUNT)); do \
		echo "$(M) run $$i of $(BENCH_COUNT)"; \
		(cd "$$base" && $(GO) test $(BENCH_FLAGS) -count 1 ./...) >> $(BENCH_DIR)/base.txt; \
		$(GO) test $(BENCH_FLAGS) -count 1 ./... >> $(BENCH_DIR)/worktree.txt; \
	done
	$Q $(BENCHSTAT) $(BENCH_BASE)=$(BENCH_DIR)/base.txt worktree=$(BENCH_DIR)/worktree.txt

## Helpers

.PHONY: go-version
go-version: ## Print go version used in this makefile
	$Q echo $(GO)

.PHONY: fmt
fmt: ## Format code
	$(info $(M) running $(GOFMT)) @
	$Q $(GOFMT) -l -w .

.PHONY: doc
doc: ## Print package documentation
	$(info $(M) running go doc) @
	$Q $(GO) doc -all .

.PHONY: help
help: ## Print this
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

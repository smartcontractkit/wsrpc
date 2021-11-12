# Run the go linter
.PHONY: lint
lint:
	golangci-lint run

# Attempts to fix all linting issues where it can
.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix

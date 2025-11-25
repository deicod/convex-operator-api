.PHONY: test vet lint ci

test:
	go test ./...

vet:
	go vet ./...

lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint is not installed. See https://golangci-lint.run/welcome/install/ for options."; \
		exit 1; \
	fi
	golangci-lint run ./...

ci: test vet lint

alias b := build
alias f := format
alias t := test

default: check

# Build the project
build:
  go build -o bin/sval .

# Lint without modifying
lint:
  gofmt -l . | tee /dev/stderr | (! grep -q .)
  go vet ./...

# Lint and test
check: lint test

# Build, lint, and test
ci: build lint test

# Format the project
format:
  gofmt -w .

# Run the project
run *args:
  go run main.go {{args}}

# Run all tests
test *args:
  go test ./... {{args}}

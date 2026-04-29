alias b := build
alias f := format
alias t := test

default: ci

# Build the project
build:
  go build ./...

# Check formatting without modifying files
lint:
  gofmt -l . | tee /dev/stderr | (! grep -q .)

# Run continuous integration checks
ci: lint test

# Format the project
format:
  gofmt -w .

# Run the project
run *args:
  go run main.go {{args}}

# Run all tests in the project
test *args:
  go test ./... {{args}}

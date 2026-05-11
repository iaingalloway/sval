version := "dev"

alias f := format
alias t := test

default: check

# Build binaries for all target platforms
publish:
  mkdir -p bin
  CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w -X main.version={{version}}" -o bin/sval-linux-amd64 .
  CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w -X main.version={{version}}" -o bin/sval-linux-arm64 .
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.version={{version}}" -o bin/sval-windows-amd64.exe .

# Lint without modifying
lint:
  gofmt -l . | tee /dev/stderr | (! grep -q .)
  go vet ./...

# Lint and test
check: lint test

# Lint, test, and publish
ci: lint test publish

# Format the project
format:
  gofmt -w .

# Run the project
run *args:
  go run main.go {{args}}

# Run all tests
test *args:
  go test ./... {{args}}

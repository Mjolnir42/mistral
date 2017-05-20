all: validate

validate:
	@go build ./...
	@go vet ./cmd/...
	@go vet ./lib/...
	@go tool vet -shadow cmd/mistral/
	@go tool vet -shadow lib/mistral/
	@golint ./cmd/...
	@golint ./lib/...
	@ineffassign cmd/mistral/
	@ineffassign lib/mistral/

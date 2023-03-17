
.PHONY: mocks
mocks:
	mockgen -source=automa.go -self_package github.com/leninmehedy/automa -package automa > mocks.go

.PHONY: build
build: mocks test

.PHONY: test
test:
	go clean -testcache
	go test -failfast -race -covermode=atomic -coverprofile coverage.out ./...

.PHONY: coverage
coverage: test
	go tool cover -html=coverage.out
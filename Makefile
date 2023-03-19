BIN="./bin"
SRC=$(shell find . -name "*.go")

ifeq (, $(shell which golangci-lint))
$(warning "could not find golangci-lint in $(PATH), run:  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.52.0")
endif

.PHONY: fmt lint test install_deps clean

default: all

all: fmt test

fmt:
	$(info ******************** checking formatting ********************)
	@test -z $(shell gofmt -l $(SRC)) || (gofmt -d $(SRC); exit 1)

lint:
	$(info ******************** running lint tools ********************)
	golangci-lint run -v

richtest: install_deps
	$(info ******************** running tests with kyoh86/richgo ********************)
	go clean -testcache
	richgo test -failfast -race -covermode=atomic -coverprofile coverage.out.tmp ./...
	cat coverage.out.tmp | grep -v "mock*" > coverage.out  # skip the coverage report for generated files

install_deps:
	$(info ******************** downloading dependencies ********************)
	go get -v ./...
	go mod tidy

clean:
	rm -rf $(BIN)

.PHONY: mocks
mocks:
	$(info ******************** generating mocks ********************)
	mockgen -source=automa.go -self_package github.com/leninmehedy/automa -package automa > mocks.go

.PHONY: test
test: install_deps mocks
	$(info ******************** running tests ********************)
	go clean -testcache
	go test -failfast -race -covermode=atomic -coverprofile coverage.out.tmp ./...
	cat coverage.out.tmp | grep -v "mock*" > coverage.out  # skip the coverage report for generated files

.PHONY: coverage
coverage: test
	$(info ******************** running coverage ********************)
	go tool cover -html=coverage.out
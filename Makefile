BIN="./bin"
SRC=$(shell find . -name "*.go")

ifeq (, $(shell which golangci-lint))
$(warning "could not find golangci-lint in $(PATH), run:  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.52.0")
endif

.PHONY: fmt lint test install_deps clean

default: all

all: fmt lint test

fmt:
	$(info ******************** checking formatting ********************)
	@test -z $(shell gofmt -l $(SRC)) || (gofmt -d $(SRC); exit 1)

lint:
	$(info ******************** running lint tools ********************)
	golangci-lint run -v

richtest: install_deps
	$(info ******************** running tests with kyoh86/richgo ********************)
	go clean -testcache
	richgo test -failfast -race -covermode=atomic -coverprofile coverage.out ./...

install_deps:
	$(info ******************** downloading dependencies ********************)
	go get -v ./...

clean:
	rm -rf $(BIN)

.PHONY: mocks
mocks:
	$(info ******************** generating mocks ********************)
	mockgen -source=automa.go -self_package github.com/automa-saga/automa -package automa > mocks.go

.PHONY: test
test: install_deps
	$(info ******************** running tests ********************)
	go clean -testcache
	#go test -failfast -race -covermode=atomic -coverprofile coverage.out ./...
	richgo test -failfast -race -covermode=atomic -coverprofile coverage.out ./...

.PHONY: coverage
coverage: test
	$(info ******************** running coverage ********************)
	go tool cover -html=coverage.out

.PHONY: release-major
release-major:
	$(info ******************** tagging major release ********************)
	cz bump --changelog --increment MAJOR

.PHONY: release-minor
release-minor:
	$(info ******************** tagging minor release ********************)
	cz bump --changelog --increment MINOR

.PHONY: release-patch
release-patch:
	$(info ******************** tagging patch release ********************)
	cz bump --changelog --increment PATCH

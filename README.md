# Automa
![test](https://github.com/automa-saga/automa/actions/workflows/test.yaml/badge.svg)
[![Commitizen friendly](https://img.shields.io/badge/commitizen-friendly-brightgreen.svg)](http://commitizen.github.io/cz-cli/)
[![semantic-release: go](https://img.shields.io/badge/semantic--release-go?logo=semantic-release)](https://github.com/semantic-release/semantic-release)
[![codecov](https://codecov.io/gh/automa-saga/automa/branch/master/graph/badge.svg?token=DMRN5J6TJW)](https://codecov.io/gh/automa-saga/automa)

Automa is a Saga Workflow Engine for Go, designed for sequential and transactional business processes. It implements the choreography Saga pattern without a centralized message broker: each step calls the next on success or triggers rollback on error.

The name `automa` is derived from the word `automate`.

## Features

- Sequential execution of workflow steps
- Automatic rollback on error
- Compensating actions for non-reversible steps
- Step-level execution reporting
- Extensible step interface

## Getting Started

**Note:** API may change before v1.0.0.

### Installation

```sh
go get -u github.com/automa-saga/automa
```

See an [example](https://github.com/automa-saga/automa/blob/master/docs/example/example.go) in the example directory. 

## Development
 - `task test` runs the tests (install `task` tool: https://taskfile.dev/installation/).
 - In order to build example, do `cd docs/examples && go build`. Then the example can be then run using `./example`.

## Contribution
Any feedback, comment and contributions are very much welcome. 

Developers are encouraged to adopt the usual open source development practices with a PR and sign-off as well as 
verified signed commits. Developers are also encouraged to use [commitizen](https://commitizen-tools.github.io/commitizen/) 
for commits messages.

Please note the PR will be squashed merge to master with commitizen format for the PR title. So even if commitizen is not
used for individual commits in the PR, the repository maintainer are requested to ensure that the PR title follows 
commitizen format before squash-merging the PR.

For beginners use [this](https://github.com/firstcontributions/first-contributions) guide as a start.

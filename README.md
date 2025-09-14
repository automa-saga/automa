# Automa
Automa is a Saga Workflow Engine for Go, designed for sequential and transactional business processes. 

The name `automa` is derived from the word `automate`.

## Features

- Sequential execution of workflow steps
- Automatic rollback on error
- Compensating actions for non-reversible steps
- Step-level execution reporting (with JSON/YAML marshalling support)
- Extensible step interface

## Getting Started

**Note:** API may change before v1.0.0.

### Installation

```sh
go get -u github.com/automa-saga/automa
```

See an [example](https://github.com/automa-saga/automa/blob/master/examples) in the examples directory. 

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

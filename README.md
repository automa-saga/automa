# Automa
![test](https://github.com/automa-saga/automa/actions/workflows/test.yaml/badge.svg)
[![Commitizen friendly](https://img.shields.io/badge/commitizen-friendly-brightgreen.svg)](http://commitizen.github.io/cz-cli/)
[![semantic-release: go](https://img.shields.io/badge/semantic--release-go?logo=semantic-release)](https://github.com/semantic-release/semantic-release)
[![codecov](https://codecov.io/gh/automa-saga/automa/branch/master/graph/badge.svg?token=DMRN5J6TJW)](https://codecov.io/gh/automa-saga/automa)

Automa is a Saga Workflow Engine. It is designed to be used for sequential and transactional business processes. It 
implements the choreography Saga pattern. The difference with the traditional
[choreography pattern](https://learn.microsoft.com/en-us/azure/architecture/reference-architectures/saga/saga) is that 
this does not use a centralized message broker, rather each step calls the next step on success or undo previous on 
error. 

The name `automa` is derived from the word `automate`.

All steps are executed sequentially in the Automa Workflow. On success, it moves forward sequentially and on error it moves
backward sequentially. Note that some steps cannot be rollback in reality, for example if an email has been sent. In that
case, some form of compensating behaviour should be implemented, for example, it should send another compensating email 
in order to void the notification email that was sent before.

Apart from Saga workflow pattern, Automa also supports generating a report of the execution for every step in the workflow. 
A report data model can be found in file [reports.go](https://github.com/automa-saga/automa/blob/master/reports.go). 
Developers need to populate a Report object in every `Run` and `Rollback` method as shown in the example. 

## Usage
See an [example](https://github.com/automa-saga/automa/blob/master/example/example.go) in the example directory. As shown 
in the example, each step can have its own internal cache to help implementing the rollback mechanism.

## Development
 - `make test` runs the tests. 
 - In order to build example, do `cd examples && go build`. Then the example can be then run using `./examples/example`.

## Contribution
Any feedback, comment and contributions are very much welcome. 

Developers are encouraged to adopt the usual open source development practices with a PR and sign-off as well as 
verified signed commits. Developers are also encouraged to use [commitizen](https://commitizen-tools.github.io/commitizen/) 
for commits messages.

Please note the PR will be squashed merge to master with commitizen format for the PR title. So even if commitizen is not
used for individual commits in the PR, the repository maintainer are requested to ensure that the PR title follows 
commitizen format before squash-merging the PR.

For beginners use [this](https://github.com/firstcontributions/first-contributions) guide as a start.

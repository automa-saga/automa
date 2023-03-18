# Automa

Automa is a Saga Workflow Engine to automate a sequential business process with transactional attribute. It implements 
the choreography pattern. The name `automa` is derived from the word `automate`.

All steps are run sequentially in the Workflow. 

If a step causes an error, it rolls back from that step and executes backward to the fist step. Therefore, it can be used 
to implement atomic transaction like behaviour for business workflows.

Apart from Saga workflow pattern, it also supports generating a report of the execution for every steps in the workflow. 
A report data model can be found in file [reports.go](https://github.com/leninmehedy/automa/blob/master/reports.go).

## Usage
See an [example](https://github.com/leninmehedy/automa/blob/master/example/main.go) in the example directory. As shown 
in the example, each step can have its own internal cache to help implementing the rollback mechanism.

## Execution Report
Automa allows generating execution report of every step. Developers need to populate a Report object in every `Run` 
and `Rollback` method as shown in the example. Report data model can be found in the 
file [reports.go](https://github.com/leninmehedy/automa/blob/master/reports.go).

## Development

 - `make build` or `make test` generates mocks and runs the tests. 
 - In order to build example and mocks, do `cd examples && go build`. Then the example can be then run using `./examples/example`.

## Contribution
This is the very early stage of development. So any feedback, comment and contributions are very much welcome. 
Developers are encouraged to adopt the usual open source development practices with a PR and sign-off as well as 
verified signed commits. 

For beginners use [this](https://github.com/firstcontributions/first-contributions) guide as a start.
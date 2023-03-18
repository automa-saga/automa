# Automa

Automa is a basic Saga Workflow Engine to automate a sequential and transactional business process. It implements 
the choreography Saga pattern in order to ensure the atomic transactional behaviour. The difference with the traditional
[choreography pattern](https://learn.microsoft.com/en-us/azure/architecture/reference-architectures/saga/saga) is that 
this does not use a centralized message broker, rather each step calls the next step on success or undo previous on 
error. Therefore, Automa is not able to parallelize the step execution (forward or undo), which is a 
desired behaviour in many of the application scenario.

The name `automa` is derived from the word `automate`.

All steps are executed sequentially in the Workflow. If a step causes an error, it rolls back from that step and executes 
backward to the fist step. Therefore, it can be used to implement atomic transaction like behaviour for business workflows. 
However, note that some steps cannot be rollback in reality, for example if an email has been sent. However, some form 
of compensating behaviour can be implemented in that case, for example, it should send another compensating email to void 
the previous notification email that was sent.

Apart from Saga workflow pattern, it also supports generating a report of the execution for every step in the workflow. 
A report data model can be found in file [reports.go](https://github.com/leninmehedy/automa/blob/master/reports.go). 
Developers need to populate a Report object in every `Run` and `Rollback` method as shown in the example. 

## Usage
See an [example](https://github.com/leninmehedy/automa/blob/master/example/example.go) in the example directory. As shown 
in the example, each step can have its own internal cache to help implementing the rollback mechanism.

## Development

 - `make build` or `make test` generates mocks and runs the tests. 
 - In order to build example and mocks, do `cd examples && go build`. Then the example can be then run using `./examples/example`.

## Contribution
This is the very early stage of development. So any feedback, comment and contributions are very much welcome. 
Developers are encouraged to adopt the usual open source development practices with a PR and sign-off as well as 
verified signed commits. 

For beginners use [this](https://github.com/firstcontributions/first-contributions) guide as a start.
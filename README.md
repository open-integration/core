# Open Integration - a pipeline execution engine

<img src="./logo.png" width="300" align="left">

[![Go Report Card](https://goreportcard.com/badge/github.com/open-integration/oi)](https://goreportcard.com/report/github.com/open-integration/oi)
[![codecov](https://codecov.io/gh/open-integration/core/branch/master/graph/badge.svg)](https://codecov.io/gh/open-integration/core)

[![asciicast](https://asciinema.org/a/312592.svg)](https://asciinema.org/a/312592)
# Install
## Homebrew 
`brew tap open-integration/oi`
`brew install oictl`

## Linux and Win
Use the [released]() binaries 


Til the project has not reached `version > 1.x.x` it may have breaking changes in the API, please use the latest version before opening issue.

# Concepts
* A compiled, binary pipeline
* State - the engine holds the state of all the tasks
* Service - a standalone binary exposing API over http2 (gRPC) that the engine can trigger, called endpoint.
	* Endpoint of a service defined by 2 files of JSON schema, `arguments.json` and `returns.json`, the engine will enforce the arguments given by a task and the output created to match the schema.
* Task - execution flow with input and output.
	* Service Task will send a request to service based on the endpoint.

## Architecture
![Diagram](docs/architecture.png)

## Dataflow
![Diagram](docs/flow-diagram.png)

## Example
use oictl to generate hello-world pipeline
* `ioctl generate pipeline`
* `go run *.go`

### Real world examples
* [JIRA](https://github.com/olegsu/jira-sync)
* [Trello](https://github.com/olegsu/trello-sync)
* [open-integration/core-ci-pipeline](https://github.com/open-integration/oi-ci-pipeline)

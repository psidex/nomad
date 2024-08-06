#!/bin/bash
#
# usage: ./run.sh command [argument ...]
#
# See https://death.andgravity.com/run-sh
# for an explanation of how it works and why it's useful.

set -ex

function proto() {
    protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. \
        --go-grpc_opt=paths=source_relative ./internal/controller/pb/controller.proto
}

function buildagent() {
    docker build -f nomad-agent.Dockerfile -t nomad-agent .
}

function buildcontroller() {
    docker build -f nomad-controller.Dockerfile -t nomad-controller .
}

function build() {
    proto
    buildagent
    buildcontroller
}

function test() {
    docker run -d --name nomad-controller --network=nomadnet nomad-controller
    sleep 1s
    docker run --rm --network=nomadnet -e NOMAD_CONTROLLER_ADDRESS=nomad-controller nomad-agent
}

"$@"

#!/bin/bash
env GOOS=linux GOARCH=amd64 go build -o ./mq.linux.amd64
env GOOS=darwin GOARCH=amd64 go build -o ./mq.darwin.amd64
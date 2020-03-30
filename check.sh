#!/bin/bash -x

set -e

export GO111MODULE=off 

go build
dep check
go test -cover $(go list ./... |grep -v pkg/persistence |grep -v pkg/api)
go test -cover github.com/dollarshaveclub/acyl/pkg/persistence
go test -cover github.com/dollarshaveclub/acyl/pkg/api
docker build -t at .
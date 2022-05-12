#!/bin/sh -x
go_version=1.18.2
repo=github.com/AirVantage/overlord
docker run --rm -v "$(pwd)":/go/src/$repo -w /go/src/$repo -e GOOS=darwin -e GOARCH=amd64 golang:$go_version go build -o overlord-darwin-amd64
docker run --rm -v "$(pwd)":/go/src/$repo -w /go/src/$repo -e GOOS=linux  -e GOARCH=amd64 golang:$go_version go build -o overlord-linux-amd64

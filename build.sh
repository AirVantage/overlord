#! /bin/bash
docker run --rm -v "$(pwd)":/go/src/github.com/AirVantage/overlord/ -w /go/src/github.com/AirVantage/overlord/ -e GOOS=darwin -e GOARCH=amd64 golang:1.13.8 sh -c "go mod download && go build -v -o overlord-darwin-amd64"
docker run --rm -v "$(pwd)":/go/src/github.com/AirVantage/overlord/ -w /go/src/github.com/AirVantage/overlord/ -e GOOS=linux -e GOARCH=amd64 golang:1.13.8 sh -c "go mod download && go build -v -o overlord-linux-amd64"

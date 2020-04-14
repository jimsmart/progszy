#!/bin/bash


# TODO This doesn't work properly on macOS.

# go build

# cd cmd/progszy
# go build

# # mkdir -p windows-amd64
# # CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -v -o windows-amd64/



# mkdir -p linux-amd64
# CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -v -o linux-amd64/ -ldflags "-linkmode external -extldflags -static"
# # ldflags come from https://github.com/mattn/go-sqlite3/issues/217


# # mkdir -p linux-arm
# # GOOS=linux GOARCH=arm go build -v -o linux-arm/

# # mkdir -p linux-386
# # GOOS=linux GOARCH=386 go build -v -o linux-386/


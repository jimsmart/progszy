#!/bin/bash

go build

cd cmd/progszy
go build

mkdir -p windows-amd64
GOOS=windows GOARCH=amd64 go build -v -o windows-amd64/

mkdir -p linux-amd64
GOOS=linux GOARCH=amd64 go build -v -o linux-amd64/

# mkdir -p linux-arm
# GOOS=linux GOARCH=arm go build -v -o linux-arm/

# mkdir -p linux-386
# GOOS=linux GOARCH=386 go build -v -o linux-386/


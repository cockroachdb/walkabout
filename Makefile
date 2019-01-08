.PHONY: build clean generate fmt install lint test

all: build

build:
	go build -ldflags "-X github.com/cockroachdb/walkabout/gen.buildID=`git describe --tags --always --dirty`" .

clean:
	go clean ./... 
	find . -name '*_walkabout*.go' -delete

generate: 
	go generate ./... 

fmt:
	go fmt ./... 

install:
	go install 

lint: generate
	go run golang.org/x/lint/golint -set_exit_status ./...
	go run honnef.co/go/tools/cmd/staticcheck -checks all ./...

test: generate
	go test -vet all ./...

release: fmt lint test build


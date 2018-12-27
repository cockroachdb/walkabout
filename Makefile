all: install

clean:
	go clean ./... 
	find . -name '*_walkabout*.go' -delete

generate: install
	go generate ./... 

fmt:
	go fmt ./... 

install:
	go install 

test: generate
	go test -vet all ./...

.PHONY: clean generate fmt install test


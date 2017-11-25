.PHONY: init update build test

init:
	go get -u github.com/golang/lint/golint github.com/golang/dep/cmd/dep

update:
	dep ensure -update

build:
	go build

test:
	go test -v ./...
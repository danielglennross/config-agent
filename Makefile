.PHONY: init update build test

init:
	go get -u github.com/golang/lint/golint github.com/golang/dep/cmd/dep

update:
	dep ensure -update

build:
	go build

test:
	exec C:/Users/Daniel/Desktop/Redis/redis-server.exe 1> /dev/null &
	go test -v ./...
.PHONY: build install test clean

build:
	go build -o terraformlib cmd/terraformlib/main.go

install:
	go install ./cmd/terraformlib

test:
	go test -v ./...

clean:
	go clean
	rm -f terraformlib

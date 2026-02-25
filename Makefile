install:
	go install .

build:
	go build -o terraformlib main.go

clean:
	go clean
	rm -f terraformlib

.PHONY: install build clean
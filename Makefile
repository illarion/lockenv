.PHONY: build test cover clean

BINARY := lockenv

build:
	go build -o $(BINARY)

install:
	go install

test:
	go test ./...

cover:
	go test -cover ./...

clean:
	rm -f $(BINARY)

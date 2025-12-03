.PHONY: build test cover clean

BINARY := lockenv

build:
	go build -o $(BINARY)

test:
	go test ./...

cover:
	go test -cover ./...

clean:
	rm -f $(BINARY)

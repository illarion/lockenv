.PHONY: build test cover clean completions install

BINARY := lockenv

build:
	go build -o $(BINARY)

install:
	go install

test:
	go test ./...

cover:
	go test -cover ./...

completions: build
	mkdir -p completions
	./$(BINARY) completion bash > completions/lockenv.bash
	./$(BINARY) completion zsh > completions/_lockenv
	./$(BINARY) completion fish > completions/lockenv.fish

clean:
	rm -f $(BINARY)

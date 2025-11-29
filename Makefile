.PHONY: build test clean install

build:
	go build -o claude-fzf ./cmd/claude-fzf

test:
	go test ./...

clean:
	rm -f claude-fzf
	rm -rf ~/.cache/claude-fzf

install: build
	mv claude-fzf /usr/local/bin/

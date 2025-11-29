.PHONY: build test clean install

build:
	go build -o claude-fzf-go ./cmd/claude-fzf

test:
	go test ./...

clean:
	rm -f claude-fzf-go
	rm -rf ~/.cache/claude-fzf

install: build
	mv claude-fzf-go /usr/local/bin/claude-fzf

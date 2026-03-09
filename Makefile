.PHONY: build run test lint clean

build:
	go build -o bot ./cmd/bot/

run:
	go run ./cmd/bot/

test:
	go test ./... -v -count=1

lint:
	go vet ./...

clean:
	rm -f bot

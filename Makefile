BINARY  = ./bin/shareiscare
SRC     = ./cmd/shareiscare/
WORKER  = ./worker

# Go build
.PHONY: build run clean vet test deploy dev

build:
	go build -o $(BINARY) $(SRC)

run: build
	./$(BINARY) --dir .

run-hash: build
	./$(BINARY) --hash $(HASH) --dir $(or $(DIR),.)

vet:
	go vet $(SRC)

test:
	go test $(SRC) -v

clean:
	rm -f $(BINARY)

# Worker (requires Node >= 20)
deploy:
	cd $(WORKER) && npx wrangler deploy

dev:
	cd $(WORKER) && npx wrangler dev

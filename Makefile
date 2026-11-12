BINARY_NAME = bin/kvengine
MAIN_SRC = cmd/server/main.go

.PHONY: all build run test bench clean

all: build

build:
	@mkdir -p bin
	go build -o $(BINARY_NAME) $(MAIN_SRC)
	@echo "Build complete: $(BINARY_NAME)"

run: build
	./$(BINARY_NAME) -port 6379

test:
	go test -v -race ./tests/...

bench:
	go test -bench=. -benchmem ./tests/...

clean:
	rm -rf bin/ *.aof

ACC=acc
BIN=bin

.PHONY: build test clean 

build:
	go build -o $(BIN)/$(ACC) main.go

test:
	go test ./...

clean:
	go clean
	rm -rf $(BIN)
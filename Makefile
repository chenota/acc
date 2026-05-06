ACC=acc
BIN=bin

.PHONY: build clean

build:
	go build -o $(BIN)/$(ACC) main.go

clean:
	go clean
	rm -rf $(BIN)
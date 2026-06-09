ACC=acc
BIN=bin

.PHONY: build test testp clean 

build:
	go build -o $(BIN)/$(ACC) main.go

test:
	go test ./internal/...

testp:
	go test ./test/...

clean:
	go clean
	rm -rf $(BIN)
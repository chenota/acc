ACC=acc
BIN=bin

.PHONY: build test testp testp-wip clean 

build:
	go build -o $(BIN)/$(ACC) main.go

test:
	go test ./internal/...

testp:
	go test ./test/...

testp-wip:
	ACC_RUN_WIP=true go test ./test/...

clean:
	go clean
	rm -rf $(BIN)
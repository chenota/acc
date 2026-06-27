ACC=acc
BIN=bin

.PHONY: build test testp testp-wip clean 

build:
	go build -o $(BIN)/$(ACC) main.go

test:
	go test ./internal/...

testp:
	go test -count=1 ./test/...

testp-wip:
	ASM_ON_FAIL=true RUN_WIP=true go test -count=1 ./test/...

clean:
	go clean
	rm -rf $(BIN)
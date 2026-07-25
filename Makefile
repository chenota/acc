ACC=acc
BIN=bin

TAG ?= ~wip

.PHONY: build test testp testp-wip clean 

build:
	go build -o $(BIN)/$(ACC) main.go

test:
	go test ./internal/...

testp:
	TAG=$(TAG) go test -count=1 ./test/...

testp-wip:
	ASM_ON_FAIL=true TAG=wip go test -count=1 ./test/...

clean:
	go clean
	rm -rf $(BIN)
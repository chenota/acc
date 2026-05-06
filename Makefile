BIN := bin
ACC := $(BIN)/acc
SRC := main.go $(wildcard cmd/*) $(wildcard internal/*)

$(ACC): $(SRC)
	go build -o $(ACC)

$(BIN):
	mkdir -p $(BIN)

.PHONY: build clean

build: $(ACC)

clean:
	rm -rf $(BIN)
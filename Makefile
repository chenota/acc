BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/acc
SOURCES := $(wildcard src/*.lisp)
SYSTEM_FILE := acc.asd
SBCL := sbcl --noinform --disable-debugger

$(BIN_PATH): $(SOURCES) $(SYSTEM_FILE) | $(BIN_DIR)
	$(SBCL) --eval '(asdf:make :acc)' --quit
	mv acc $(BIN_DIR)

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

.PHONY: build test clean

build: $(BIN_PATH)

test:
	$(SBCL) --eval '(ql:quickload :acc/test)' --eval '(fiveam:run-all-tests)' --quit

clean:
	rm -rf $(BIN_DIR)
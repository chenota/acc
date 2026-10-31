package runtime

import _ "embed"

// garbage collector runtime precompiled for x64 linux
//
//go:embed c/runtime_linux.o
var object []byte

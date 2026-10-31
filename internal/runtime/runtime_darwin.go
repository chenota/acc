package runtime

import _ "embed"

// garbage collector runtime precompiled for x64 macos
//
//go:embed c/runtime_darwin.o
var object []byte

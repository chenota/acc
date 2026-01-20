# acc

[![CI](https://github.com/chenota/acc/actions/workflows/ci.yml/badge.svg)](https://github.com/chenota/acc/actions/workflows/ci.yml)
[![CI](https://raw.githubusercontent.com/chenota/acc/refs/heads/coverage/badge.svg)](https://github.com/chenota/acc/blob/main/.github/scripts/coverage.lisp)

The so back meters are off the charts (we are so back). AlexC has a lot of problems and the codebase is kind of a mess so we're starting fresh with acc, which stands for AlexC Continued. This might be the greatest thing ever made or it might end up being a worse version of Go, only time will tell. I'll update the README once I make more progress.

## Build

I've included a Makefile for building `acc` into an executable. Simply run the included build rule and you'll find `acc` in the `bin` directory.

```
make build
```

You can use the clean rule to delete the `acc` binary.

```
make clean
```

## Run

The `acc` executable expects an input file and an output file. For either of these you can use `-` to designate stdin and stdout, respectively.

| Flag         | Description       |
|--------------|-------------------|
| `-h, --help` | Display help text |

## Test

You can use the test rule for running unit tests.

```
make test
```

## Development

I'm trying out a new development strategy where I work in vertical slices, capturing a specific functionality and seeing it to the end before starting on the next feature. It's very important that I write tests to ensure that adding new features don't break existing functionality. 

### Feature 1: Main Function

I should have the ability to write a main function which can return any integer as a status code to be accessed with `$?`.

```
func main int {
    return 0;
}
```

#### Statement Grammar (PEG)

```
<Program>  := <Function>
<Function> := "func" "main" "int" "{" <Stmt> "}"
<Stmt>     := "return" <Expr> ";"
```

#### Expression Grammar (CFG)

```
<Expr> := <Int>
<Int>  := [0-9]+
```

### Feature 2: Basic Type System

It seems weird to have a type system when the only possible expression is a single integer, however the `int` from `func main int` is a fake type that's not meaningful in any way which bothers me, and I'd like to have types integrated into this project as early as possible as to avoid needing to hack a type system in later. I'm also introducing a type conversion operator for testing purposes.

```
func main int {
    return (int) 0;
}
```

#### Statement Grammar (PEG)

```
<Program>  := <Function>
<Function> := "func" "main" "int" "{" <Stmt> "}"
<Stmt>     := "return" <Expr> ";"
```

#### Expression Grammar (CFG)

```
<Expr> := <Int>
        | "(" <Type> ")" <Expr>
        | "(" <Expr> ")"
<Int>  := [0-9]+
```

#### Type Grammar (CFG)

```
<Type> := <Int>
<Int>  := "char"
        | "int16"
        | "int32"
        | "int64"
        | "int"
```
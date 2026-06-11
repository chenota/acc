# acc

The so back meter is off the charts. `acc` stands for [AlexC](https://github.com/chenota/alexc) Continued; it's a language that targets AMD64 assembly.

## Goals

The goal for this project is to create a feature-rich, compiled language that combines elements of languages like OCaml with the easy-to-read syntax and low-level control of systems languages like C. You can think of it like Go with pattern matching and a real option type.

## Building and Running

You can build `acc` with
```shell
make build
```

You can view information on how to run `acc` with
```shell
acc --help
```

## Testing

`acc` contains two types of tests: unit tests and program tests.

The unit tests focus on individual components of the compiler and are most helpful as a development tool for me so I can be confident that individual components are functioning as expected. You can run them with
```shell
make test
```

The program tests are a little more interesting; they actually use `acc` to compile a binary from source code, run it, and validate the output against a set of golden files. These tests are super helpful for making sure new features I introduce into the language don't introduce regressions elsewhere, and act as the final gate on whether or not I can say a vertical slice is complete. You can run them with
```shell
make testp
```

## Vertical Slices

To help with maintainability, I'm planning to write this compiler in a series of vertical slices that each introduce a specific and well-tested feature. Once a feature is introduced, I cannot break it or else I'm FIRED! For each vertical slice I'll provide a goal and an updated grammar for the various parts of the language.
 
### Vertical Slice 1: Exit Code [Complete] 

The first goal of this language is to have a main function that can return an exit code. This is really groundbreaking stuff!

#### Program Grammar (PEG)

```
<Program>   := <Function>
<Function>  := "fun" "main" "(" ")" "->" <Type> <Block>
<Block>     := "{" <Statement> "}"
<Statement> := "return" <Expression> ";"
```

#### Expression Grammar (CFG)

```
<Expression> := <Integer>
<Integer>    := /[0-9]+/
```

#### Type Grammar (CFG)

```
<Type> := <Atom>
<Atom> := "int"
```

### Vertical Slice 2: Constant Arithmetic

Return an exit code from the result of an arithmetic expression; this is deceptively simple since `acc` is going to implement constant folding but it's a necessary setup for the future.

#### Expression Grammar (CFG)

```
<Expression> := <Add>
<Add>        := <Add> "+" <Mul>
              | <Add> "-" <Mul>
              | <Mul>
<Mul>        := <Mul> "*" <Atom>
              | <Mul> "/" <Atom>
              | <Atom>
<Atom>       := <Integer>
              | "(" <Expression> ")"
<Integer>    := /[0-9]+/
```
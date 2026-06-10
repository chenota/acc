# acc

The so back meter is off the charts. `acc` stands for [AlexC](https://github.com/chenota/alexc) Continued; it's a language that targets AMD64 assembly.

## Goals

The goal for this project is to create a feature-rich, compiled language that combines elements of languages like OCaml with the easy-to-read syntax and low-level control of systems languages like C. You can think of it like Go with pattern matching and a real option type.

## Vertical Slices

To help with maintainability, I'm planning to write this compiler in a series of vertical slices that each introduce a specific and well-tested feature. Once a feature is introduced, I cannot break it or else I'm FIRED! For each vertical slice I'll provide a goal and an updated grammar for the various parts of the language.

### [Done] Vertical Slice 1: Exit Code

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
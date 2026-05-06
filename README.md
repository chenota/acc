# acc

The so back meters are off the charts (we are so back). `acc` stands for AlexC Continued and is a rebirth of the [alexc](github.com/chenota/alexc) project, which I've decided to step away from.

## Goals

The goal for this project is to create a feature-rich, compiled language that combines elements of functional/ML languages like OCaml with the easy-to-read syntax and low-level control of systems languages like C. You can think of it like Go with pattern matching and a real option type, or like dumbed-down Rust because no way I'm going to write something as complex as Rust.

## Vertical Slices

To help with maintainability, I'm planning to write this compiler in a series of vertical slices that each introduce a specific and well-tested feature. Once a feature is introduced, it cannot be broken or else I'm FIRED! This strategy may be a little weird at first when I'm just getting the groundwork going, but overall I think it's going to work out well.

For each vertical slice I'll provide a goal and an updated grammar for the various parts of the language, which will end up with multiple grammars.

### Vertical Slice 1: Exit Code

The first goal of this language is to have a main function that can return an exit code. This is really groundbreaking stuff!

#### Program Grammar (PEG)

```
<Program>   := <Function>
<Function>  := "fun" "main" <Type> <Block>
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
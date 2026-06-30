# acc

The so back meter is off the charts. `acc` stands for [AlexC](https://github.com/chenota/alexc) Continued; it's a language that targets x64 assembly.

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

### Vertical Slice 2: Constant Arithmetic [Complete]

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

### Vertical Slice 3: Variables [Complete]

`acc` at this point is still stuck with its only output being an exit code. I'd like to work towards being able to do file output via a format print, but to get to that point `acc` needs a couple of foundational constructs with variables being one of them. I've made the type in a declaration optional since the bidirectional type system naturally supports inference very well so it's not a huge lift to add support now.

#### Program Grammar (PEG)

```
<Statement> := "return" <Expression> ";"
             | "let" <Ident> <Type>? "=" <Expression> ";"
             | <Ident> "=" <Expression> ";"
```

#### Expression Grammar (CFG)

```
<Atom> := <Ident>
```

### Vertical Slice 4: Negation [Complete]

I want to get negative numbers out of the way now and they can help us introduce some foundational concepts like unary operations.

#### Expression Grammar (CFG)

```
<Mul>   := <Mul> "*" <Atom>
         | <Mul> "/" <Atom>
         | <Unary>
<Unary> := "-" <Atom>
         | <Atom>
```

### Vertical Slice 5: Assignment Operators [Complete]

Another low-hanging fruit I'd like to knock out is assignment operators since everything is pretty much in place for them already. Can you tell I'm putting off functions since those'll be difficult?

#### Program Grammar (PEG)

```
<Program>   := <Function>
<Function>  := "fun" "main" "(" ")" "->" <Type> <Block>
<Block>     := "{" <Statement> "}"
<Statement> := "return" <Expression> ";"
             | "let" <Ident> <Type>? "=" <Expression> ";"
             | <Ident> "=" <Expression> ";"
             | <Ident> ("+=" | "-=" | "*=" | "/=") <Expression> ";"
```

### Vertical Slice 6: Global Functions [Work in Progress]

We can build on Vertical Slice 3 and add the last foundational construct we need before introducing a format print by adding functions.

#### Program Grammar (PEG)

```
<Program>    := <Function>+
<Function>   := "fun" <Ident> "(" <Paramlist> ")" "->" <Type> <Block>
<Block>      := "{" <Statement> "}"
<Statement>  := "return" <Expression> ";"
              | "let" <Ident> <Type>? "=" <Expression> ";"
              | <Ident> "=" <Expression> ";"
              | <Ident> ("+=" | "-=" | "*=" | "/=") <Expression> ";"
```

#### Expression Grammar (CFG)

```
<Unary>      := "-" <Call>
              | <Call>
<Call>       := <Atom> "(" <Exprlist> ")"
              | <Atom>
```

### Vertical Slice 7: Lambda Functions [Not Started]

Lambda functions let `acc` use functions as values.

### Vertical Slice 8: String Literals and File Output [Not Started]

With functions and variables out of the way, we can finally add a format print which greatly expands the usefulness of the `acc` language.
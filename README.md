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
Program   <- Function
Function  <- "fun" "main" "(" ")" "->" Type Block
Block     <- "{" Statement "}"
Statement <- "return" Expression ";"
```

#### Expression Grammar (EBNF)

```
Expression = Integer ;
```

#### Type Grammar (EBNF)

```
Type = "int" ;
```

### Vertical Slice 2: Constant Arithmetic [Complete]

Return an exit code from the result of an arithmetic expression; this is deceptively simple since `acc` is going to implement constant folding but it's a necessary setup for the future.

#### Expression Grammar (EBNF)

```
Expression = Add ;
Add        = Mul { ( "+" | "-" ) Mul } ;
Mul        = Atom { ( "*" | "/" ) Atom } ;
Atom       = Integer
           | "(" Expression ")" ;
```

### Vertical Slice 3: Variables [Complete]

`acc` at this point is still stuck with its only output being an exit code. I'd like to work towards being able to do file output via a format print, but to get to that point `acc` needs a couple of foundational constructs with variables being one of them. I've made the type in a declaration optional since the bidirectional type system naturally supports inference very well so it's not a huge lift to add support now.

#### Program Grammar (PEG)

```
Program     <- Function
Function    <- "fun" "main" "(" ")" "->" Type Block
Block       <- "{" Statement* "}"
Statement   <- Declaration / Assignment / Return
Declaration <- "let" Ident Type? "=" Expression ";"
Assignment  <- Ident "=" Expression ";"
Return      <- "return" Expression ";"
```

#### Expression Grammar (EBNF)

```
Expression = Add ;
Add        = Mul { ( "+" | "-" ) Mul } ;
Mul        = Atom { ( "*" | "/" ) Atom } ;
Atom       = Integer
           | Ident
           | "(" Expression ")" ;
```

### Vertical Slice 4: Negation [Complete]

I want to get negative numbers out of the way now and they can help us introduce some foundational concepts like unary operations.

#### Expression Grammar (EBNF)

```
Expression = Add ;
Add        = Mul { ( "+" | "-" ) Mul } ;
Mul        = Unary { ( "*" | "/" ) Unary } ;
Unary      = "-" Unary
           | Atom ;
Atom       = Integer
           | Ident
           | "(" Expression ")" ;
```

### Vertical Slice 5: Assignment Operators [Complete]

Another low-hanging fruit I'd like to knock out is assignment operators since everything is pretty much in place for them already. Can you tell I'm putting off functions since those'll be difficult?

#### Program Grammar (PEG)

```
Program      <- Function
Function     <- "fun" "main" "(" ")" "->" Type Block
Block        <- "{" Statement* "}"
Statement    <- Declaration / Assignment / AssignmentOp / Return
Declaration  <- "let" Ident Type? "=" Expression ";"
Assignment   <- Ident "=" Expression ";"
AssignmentOp <- Ident ( "+=" / "-=" / "*=" / "/=" ) Expression ";"
Return       <- "return" Expression ";"
```

### Vertical Slice 6: Global Functions [Complete]

We can build on Vertical Slice 3 and add the last foundational construct we need before introducing a format print by adding functions.

#### Program Grammar (PEG)

```
Program      <- Function+
Function     <- "fun" Ident "(" Paramlist ")" "->" Type Block
Paramlist    <- ( Param ( "," Param )* )?
Param        <- Ident Type
Block        <- "{" Statement* "}"
Statement    <- Declaration / Assignment / AssignmentOp / Return
Declaration  <- "let" Ident Type? "=" Expression ";"
Assignment   <- Ident "=" Expression ";"
AssignmentOp <- Ident ( "+=" / "-=" / "*=" / "/=" ) Expression ";"
Return       <- "return" Expression ";"
```

#### Expression Grammar (EBNF)

```
Expression = Add ;
Add        = Mul { ( "+" | "-" ) Mul } ;
Mul        = Unary { ( "*" | "/" ) Unary } ;
Unary      = "-" Unary
           | Postfix ;
Postfix    = Atom { "(" [ Exprlist ] ")" } ;
Exprlist   = Expression { "," Expression } ;
Atom       = Integer
           | Ident
           | "(" Expression ")" ;
```

### Vertical Slice 7: Pointers [Complete]

We need referenced values to make closures work.

#### Program Grammar (PEG)

```
Program       <- Function+
Function      <- "fun" Ident "(" Paramlist ")" ( "->" Type )? Block
Paramlist     <- ( Param ( "," Param )* )?
Param         <- Ident Type
Block         <- "{" Statement* "}"
Statement     <- Declaration / Assignment / AssignmentOp / Return / CallStatement
Declaration   <- "let" Ident Type? "=" Expression ";"
Assignment    <- Expression "=" Expression ";"
AssignmentOp  <- Expression ( "+=" / "-=" / "*=" / "/=" ) Expression ";"
Return        <- "return" Expression? ";"
CallStatement <- Expression ";"
```

#### Expression Grammar (EBNF)

```
Expression = Add ;
Add        = Mul { ( "+" | "-" ) Mul } ;
Mul        = Unary { ( "*" | "/" ) Unary } ;
Unary      = ( "-" | "&" | "*" ) Unary
           | Postfix ;
Postfix    = Atom { "(" [ Exprlist ] ")" } ;
Exprlist   = Expression { "," Expression } ;
Atom       = Integer
           | Ident
           | "(" Expression ")" ;
```

#### Type Grammar (EBNF)

```
Type = "*" Type
     | "int" ;
```

### Vertical Slice 8: Tuples [Work in Progress]

I was halfway through implementing closures when I realized I need some kind of fielded type for storing captured values. Yikes! Decided to go with tuples since they're all I need and the easiest to get off the ground.

#### Expression Grammar (EBNF)

```
Expression = Add ;
Add        = Mul { ( "+" | "-" ) Mul } ;
Mul        = Unary { ( "*" | "/" ) Unary } ;
Unary      = ( "-" | "&" | "*" ) Unary
           | Postfix ;
Postfix    = Atom { Call | Field } ;
Call       = "(" [ Exprlist ] ")" ;
Field      = "." ( Integer | Ident ) ;
Exprlist   = Expression { "," Expression } [ "," ] ;
Atom       = Integer
           | Ident
           | "(" [ Exprlist ] ")" ;
```

#### Type Grammar (EBNF)

```
Type     = "*" Type
         | "(" [ Typelist ] ")"
         | "int" ;
Typelist = Type { "," Type } [ "," ] ;
```

### Vertical Slice 9: Closures [Work in Progress]

Closures let `acc` use functions as values.

#### Expression Grammar (EBNF)

```
Expression = Add ;
Add        = Mul { ( "+" | "-" ) Mul } ;
Mul        = Unary { ( "*" | "/" ) Unary } ;
Unary      = ( "-" | "&" | "*" ) Unary
           | Postfix ;
Postfix    = Atom { Call | Field } ;
Call       = "(" [ Exprlist ] ")" ;
Field      = "." ( Integer | Ident ) ;
Exprlist   = Expression { "," Expression } [ "," ] ;
Atom       = Integer
           | Ident
           | Lambda
           | "(" [ Exprlist ] ")" ;
Lambda     = "fun" "(" Paramlist ")" [ "->" Type ] Block ;
```

`Paramlist` and `Block` are the same rules as in the program grammar.

#### Type Grammar (EBNF)

```
Type     = "fun" "(" [ Typelist ] ")" [ "->" Type ]
         | "*" Type
         | "(" [ Typelist ] ")"
         | "int" ;
Typelist = Type { "," Type } [ "," ] ;
```

### Vertical Slice 10: String Literals and File Output [Not Started]

With functions and variables out of the way, we can finally add a format print which greatly expands the usefulness of the `acc` language.

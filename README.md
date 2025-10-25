# acc

[![CI](https://github.com/chenota/acc/actions/workflows/test.yaml/badge.svg)](https://github.com/chenota/acc/actions/workflows/test.yaml)

The so back meters are off the charts (we are so back). AlexC has a lot of problems and the codebase is kind of a mess so we're starting fresh with acc, which stands for AlexC Continued. This might be the greatest thing ever made or it might end up being a worse version of Go, only time will tell. I'll update the README once I make more progress.

## Development Methodology

I'm trying out a new development methodology where I work in vertical slices, capturing a specific functionality and seeing it to the end before starting on the next feature. It's very important that I write tests to ensure that adding new features don't break existing functionality. 

### Feature 1: Main Function

I should have the ability to write a main function which can return any integer as a status code to be accessed with `$?`.

```
func main int {
    return 0;
}
```

```
<Program>  -> <Function>
<Function> -> "func" "main" "int" "{" "return" <Int> ";" "}"
<Int>      -> [0-9]+
```

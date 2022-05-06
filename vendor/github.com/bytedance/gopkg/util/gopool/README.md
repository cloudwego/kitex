# gopool

## Introduction

`gopool` is a high-performance goroutine pool which aims to reuse goroutines and limit the number of goroutines.

It is an alternative to the `go` keyword.

## Features

- High Performance
- Auto-recovering Panics
- Limit Goroutine Numbers
- Reuse Goroutine Stack

## QuickStart

Just replace your `go func(){...}` with `gopool.Go(func(){...})`.

old:
```go
go func() {
	// do your job
}()
```

new:
```go
gopool.Go(func(){
	/// do your job
})
```

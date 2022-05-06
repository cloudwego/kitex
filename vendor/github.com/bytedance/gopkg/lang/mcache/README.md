# mcache

## Introduction

`mcache` is a memory pool which preserves memory in `sync.Pool` to improve malloc performance.

The usage is quite simple: call `mcache.Malloc` directly, and don't forget to `Free` it! 

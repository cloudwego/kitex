# fastrand

Fastest pseudo-random number generator in Go. 

This generator actually come from Go runtime per-M structure, and the init-seed is provided by Go runtime, which means you can't add your own seed, but these methods scales very well on multiple cores.

The generator passes the SmallCrush suite, part of TestU01 framework: http://simul.iro.umontreal.ca/testu01/tu01.html

pseudo-random paper: https://www.jstatsoft.org/article/view/v008i14/xorshift.pdf

fast-modulo-reduction: https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/



## Compare to math/rand

- Much faster (~5x faster for single-core; ~200x faster for multiple-cores)
- Scales well on multiple cores
- **Not** provide a stable value stream (can't inject init-seed)
- Fix bugs in math/rand `Float64` and `Float32`  (since no need to preserve the value stream)



## Bencmark

Go version: go1.15.6 linux/amd64

CPU: AMD 3700x(8C16T), running at 3.6GHz

OS: ubuntu 18.04

MEMORY: 16G x 2 (3200MHz)



##### multiple-cores

```
name                                 time/op
MultipleCore/math/rand-Int31n(5)-16   152ns ± 0%
MultipleCore/fast-rand-Int31n(5)-16  0.40ns ± 0%
MultipleCore/math/rand-Int63n(5)-16   167ns ± 0%
MultipleCore/fast-rand-Int63n(5)-16  3.04ns ± 0%
MultipleCore/math/rand-Float32()-16   143ns ± 0%
MultipleCore/fast-rand-Float32()-16  0.40ns ± 0%
MultipleCore/math/rand-Uint32()-16    133ns ± 0%
MultipleCore/fast-rand-Uint32()-16   0.23ns ± 0%
MultipleCore/math/rand-Uint64()-16    140ns ± 0%
MultipleCore/fast-rand-Uint64()-16   0.56ns ± 0%
```



##### single-core

```
name                               time/op
SingleCore/math/rand-Int31n(5)-16  15.5ns ± 0%
SingleCore/fast-rand-Int31n(5)-16  3.91ns ± 0%
SingleCore/math/rand-Int63n(5)-16  24.4ns ± 0%
SingleCore/fast-rand-Int63n(5)-16  24.4ns ± 0%
SingleCore/math/rand-Uint32()-16   10.9ns ± 0%
SingleCore/fast-rand-Uint32()-16   2.86ns ± 0%
SingleCore/math/rand-Uint64()-16   11.3ns ± 0%
SingleCore/fast-rand-Uint64()-16   5.88ns ± 0%
```



##### compare to other repo

(multiple-cores)

rand-1 -> this repo

rand-2 -> https://github.com/valyala/fastrand

```
name                         time/op
VS/fastrand-1-Uint32()-16    0.23ns ± 0%
VS/fastrand-2-Uint32()-16    3.04ns ±23%
VS/fastrand-1-Uint32n(5)-16  0.23ns ± 1%
VS/fastrand-2-Uint32n(5)-16  3.13ns ±32%
```


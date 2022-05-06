/*
 * Copyright 2022 ByteDance Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package decoder

import (
    `math/bits`
    `unsafe`

    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/binary/defs`
)

type (
    _skipbuf_t [defs.StackSize]SkipItem
)

var _SkipSizeFixed = [256]int {
    defs.T_bool   : 1,
    defs.T_i8     : 1,
    defs.T_double : 8,
    defs.T_i16    : 2,
    defs.T_i32    : 4,
    defs.T_i64    : 8,
}

const (
    _T_map_pair defs.Tag = 0xff
)

func u32be(s unsafe.Pointer) int {
    return int(bits.ReverseBytes32(*(*uint32)(s)))
}

func stpop(s *_skipbuf_t, p *int) bool {
    if s[*p].N == 0 {
        *p--
        return true
    } else {
        s[*p].N--
        return false
    }
}

func stadd(s *_skipbuf_t, p *int, t defs.Tag) bool {
    if *p++; *p >= defs.StackSize {
        return false
    } else {
        s[*p].T, s[*p].N = t, 0
        return true
    }
}

func mvbuf(s *unsafe.Pointer, n *int, r *int, nb int) {
    *n = *n - nb
    *r = *r + nb
    *s = unsafe.Pointer(uintptr(*s) + uintptr(nb))
}

func do_skip(st *_skipbuf_t, s unsafe.Pointer, n int, t defs.Tag) (rv int) {
    sp := 0
    st[0].T = t

    /* run until drain */
    for sp >= 0 {
        switch st[sp].T {
            default: {
                return ETAG
            }

            /* simple fixed types */
            case defs.T_bool   : fallthrough
            case defs.T_i8     : fallthrough
            case defs.T_double : fallthrough
            case defs.T_i16    : fallthrough
            case defs.T_i32    : fallthrough
            case defs.T_i64    : {
                if nb := _SkipSizeFixed[st[sp].T]; n < nb {
                    return EEOF
                } else {
                    stpop(st, &sp)
                    mvbuf(&s, &n, &rv, nb)
                }
            }

            /* strings & binaries */
            case defs.T_string: {
                if n < 4 {
                    return EEOF
                } else if nb := u32be(s) + 4; n < nb {
                    return EEOF
                } else {
                    stpop(st, &sp)
                    mvbuf(&s, &n, &rv, nb)
                }
            }

            /* structs */
            case defs.T_struct: {
                var nb int
                var vt defs.Tag

                /* must have at least 1 byte */
                if n < 1 {
                    return EEOF
                }

                /* check for end of tag */
                if vt = *(*defs.Tag)(s); vt == 0 {
                    stpop(st, &sp)
                    mvbuf(&s, &n, &rv, 1)
                    continue
                }

                /* check for tag value */
                if !vt.IsWireTag() {
                    return ETAG
                }

                /* fast-path for primitive fields */
                if nb = _SkipSizeFixed[vt]; nb != 0 {
                    if n < nb + 3 {
                        return EEOF
                    } else {
                        mvbuf(&s, &n, &rv, nb + 3)
                        continue
                    }
                }

                /* must have more than 3 bytes (fields cannot have a size of zero), also skip the field ID cause we don't care */
                if n <= 3 {
                    return EEOF
                } else if !stadd(st, &sp, vt) {
                    return ESTACK
                } else {
                    mvbuf(&s, &n, &rv, 3)
                }
            }

            /* maps */
            case defs.T_map: {
                var np int
                var kt defs.Tag
                var vt defs.Tag

                /* must have at least 6 bytes */
                if n < 6 {
                    return EEOF
                }

                /* get the element type and count */
                kt = (*[2]defs.Tag)(s)[0]
                vt = (*[2]defs.Tag)(s)[1]
                np = u32be(unsafe.Pointer(uintptr(s) + 2))

                /* check for tag value */
                if !kt.IsWireTag() || !vt.IsWireTag() {
                    return ETAG
                }

                /* empty map */
                if np == 0 {
                    stpop(st, &sp)
                    mvbuf(&s, &n, &rv, 6)
                    continue
                }

                /* fast path for fixed key and value */
                if nk, nv := _SkipSizeFixed[kt], _SkipSizeFixed[vt]; nk != 0 && nv != 0 {
                    if nb := np * (nk + nv) + 6; n < nb {
                        return EEOF
                    } else {
                        stpop(st, &sp)
                        mvbuf(&s, &n, &rv, nb)
                        continue
                    }
                }

                /* set to parse the map pairs */
                st[sp].K = kt
                st[sp].V = vt
                st[sp].T = _T_map_pair
                st[sp].N = uint32(np) * 2 - 1
                mvbuf(&s, &n, &rv, 6)
            }

            /* map pairs */
            case _T_map_pair: {
                if vt := st[sp].V; stpop(st, &sp) || st[sp].N & 1 != 0 {
                    if !stadd(st, &sp, vt) {
                        return ESTACK
                    }
                } else {
                    if !stadd(st, &sp, st[sp].K) {
                        return ESTACK
                    }
                }
            }

            /* sets and lists */
            case defs.T_set  : fallthrough
            case defs.T_list : {
                var nv int
                var et defs.Tag

                /* must have at least 5 bytes */
                if n < 5 {
                    return EEOF
                }

                /* get the element type and count */
                et = *(*defs.Tag)(s)
                nv = u32be(unsafe.Pointer(uintptr(s) + 1))

                /* check for tag value */
                if !et.IsWireTag() {
                    return ETAG
                }

                /* empty sequence */
                if nv == 0 {
                    stpop(st, &sp)
                    mvbuf(&s, &n, &rv, 5)
                    continue
                }

                /* fast path for fixed types */
                if nt := _SkipSizeFixed[et]; nt != 0 {
                    if nb := nv * nt + 5; n < nb {
                        return EEOF
                    } else {
                        stpop(st, &sp)
                        mvbuf(&s, &n, &rv, nb)
                        continue
                    }
                }

                /* set to parse the elements */
                st[sp].T = et
                st[sp].N = uint32(nv) - 1
                mvbuf(&s, &n, &rv, 5)
            }
        }
    }

    /* all done */
    return
}

func emu_ccall_skip(ctx hir.CallContext) {
    if !ctx.Verify("**ii", "i") {
        panic("invalid skip call")
    } else {
        ctx.Ru(0, uint64(do_skip((*_skipbuf_t)(ctx.Ap(0)), ctx.Ap(1), int(ctx.Au(2)), defs.Tag(ctx.Au(3)))))
    }
}

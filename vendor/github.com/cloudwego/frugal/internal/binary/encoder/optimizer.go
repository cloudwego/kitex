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

package encoder

import (
    `encoding/binary`
    `fmt`
    `sort`
    `strings`
    `unsafe`

    `github.com/cloudwego/frugal/internal/rt`
    `github.com/oleiade/lane`
)

type BasicBlock struct {
    P    Program
    Src  int
    End  int
    Next *BasicBlock
    Link *BasicBlock
}

func (self *BasicBlock) Len() int {
    return self.End - self.Src
}

func (self *BasicBlock) Free() {
    q := lane.NewQueue()
    m := make(map[*BasicBlock]struct{})

    /* traverse the graph with BFS */
    for q.Enqueue(self); !q.Empty(); {
        v := q.Dequeue()
        p := v.(*BasicBlock)

        /* add branch to queue */
        if p.Link != nil {
            if _, ok := m[p.Link]; !ok {
                q.Enqueue(p.Link)
            }
        }

        /* clear branch, and add to free list */
        m[p] = struct{}{}
        p.Link = nil
    }

    /* reset and free all the nodes */
    for p := range m {
        p.Next = nil
        freeBasicBlock(p)
    }
}

func (self *BasicBlock) String() string {
    n := self.End - self.Src
    v := make([]string, n + 1)

    /* dump every instructions */
    for i := self.Src; i < self.End; i++ {
        v[i - self.Src + 1] = "    " + self.P[i].Disassemble()
    }

    /* add the entry label */
    v[0] = fmt.Sprintf("L_%d:", self.Src)
    return strings.Join(v, "\n")
}

type GraphBuilder struct {
    Pin   map[int]bool
    Graph map[int]*BasicBlock
}

func (self *GraphBuilder) scan(p Program) {
    for _, v := range p {
        if _OpBranches[v.Op] {
            self.Pin[v.To] = true
        }
    }
}

func (self *GraphBuilder) block(p Program, i int, bb *BasicBlock) {
    bb.Src = i
    bb.End = i

    /* traverse down until it hits a branch instruction */
    for i < len(p) && !_OpBranches[p[i].Op] {
        i++
        bb.End++

        /* hit a merge point, merge with existing block */
        if self.Pin[i] {
            bb.Next = self.branch(p, i)
            return
        }
    }

    /* end of basic block */
    if i == len(p) {
        return
    }

    /* also include the branch instruction */
    bb.End++
    bb.Next = self.branch(p, p[i].To)

    /* GOTO instruction doesn't technically "branch", anything
     * sits between it and the next branch target are unreachable. */
    if p[i].Op != OP_goto {
        bb.Link = bb.Next
        bb.Next = self.branch(p, i + 1)
    }
}

func (self *GraphBuilder) branch(p Program, i int) *BasicBlock {
    var ok bool
    var bb *BasicBlock

    /* check for existing basic blocks */
    if bb, ok = self.Graph[i]; ok {
        return bb
    }

    /* create a new block */
    bb = newBasicBlock()
    bb.P, bb.Next, bb.Link = p, nil, nil

    /* process the new block */
    self.Graph[i] = bb
    self.block(p, i, bb)
    return bb
}

func (self *GraphBuilder) Free() {
    rt.MapClear(self.Pin)
    rt.MapClear(self.Graph)
    freeGraphBuilder(self)
}

func (self *GraphBuilder) Build(p Program) *BasicBlock {
    self.scan(p)
    return self.branch(p, 0)
}

func (self *GraphBuilder) BuildAndFree(p Program) (bb *BasicBlock) {
    bb = self.Build(p)
    self.Free()
    return
}

type _OptimizerState struct {
    buf  []*BasicBlock
    refs map[int]int
    mask map[*BasicBlock]bool
}

func (self *_OptimizerState) visit(bb *BasicBlock) bool {
    var mm bool
    var ok bool

    /* check for duplication */
    if mm, ok = self.mask[bb]; mm && ok {
        return false
    }

    /* add to block buffer */
    self.buf = append(self.buf, bb)
    self.mask[bb] = true
    return true
}

func Optimize(p Program) Program {
    acc := 0
    ret := newProgram()
    buf := lane.NewQueue()
    ctx := newOptimizerState()
    cfg := newGraphBuilder().BuildAndFree(p)

    /* travel with BFS */
    for buf.Enqueue(cfg); !buf.Empty(); {
        v := buf.Dequeue()
        b := v.(*BasicBlock)

        /* check for duplication, and then mark as visited */
        if !ctx.visit(b) {
            continue
        }

        /* optimize each block */
        for _, pass := range _PassTab {
            pass(b)
        }

        /* add conditional branches if any */
        if b.Next != nil { buf.Enqueue(b.Next) }
        if b.Link != nil { buf.Enqueue(b.Link) }
    }

    /* sort the blocks by entry point */
    sort.Slice(ctx.buf, func(i int, j int) bool {
        return ctx.buf[i].Src < ctx.buf[j].Src
    })

    /* remap all the branch locations */
    for _, bb := range ctx.buf {
        ctx.refs[bb.Src] = acc
        acc += bb.End - bb.Src
    }

    /* adjust all the branch targets */
    for _, bb := range ctx.buf {
        if end := bb.End; bb.Src != end {
            if ins := &bb.P[end - 1]; _OpBranches[ins.Op] {
                ins.To = ctx.refs[ins.To]
            }
        }
    }

    /* grow the result buffer if needed */
    if cap(ret) < acc {
        rt.GrowSlice(&ret, acc)
    }

    /* merge all the basic blocks */
    for _, bb := range ctx.buf {
        ret = append(ret, bb.P[bb.Src:bb.End]...)
    }

    /* release the original program */
    p.Free()
    freeOptimizerState(ctx)
    return ret
}

var _PassTab = [...]func(p *BasicBlock) {
    _PASS_StaticSizeMerging,
    _PASS_SeekMerging,
    _PASS_NopElimination,
    _PASS_SizeCheckMerging,
    _PASS_LiteralMerging,
    _PASS_Compacting,
}

const (
    _NOP OpCode = 0xff
)

func init() {
    _OpNames[_NOP] = "(nop)"
}

func checksl(s *[]byte, n int) *rt.GoSlice {
    sl := (*rt.GoSlice)(unsafe.Pointer(s))
    sn := sl.Len

    /* check for length */
    if sn + n > sl.Cap {
        panic("slice overflow")
    } else {
        return sl
    }
}

func append1(s *[]byte, v byte) {
    sl := checksl(s, 1)
    sl.Set(sl.Len, v)
    sl.Len++
}

func append2(s *[]byte, v uint16) {
    sl := checksl(s, 2)
    sl.Set(sl.Len + 0, byte(v >> 8))
    sl.Set(sl.Len + 1, byte(v))
    sl.Len += 2
}

func append4(s *[]byte, v uint32) {
    sl := checksl(s, 4)
    sl.Set(sl.Len + 0, byte(v >> 24))
    sl.Set(sl.Len + 1, byte(v >> 16))
    sl.Set(sl.Len + 2, byte(v >> 8))
    sl.Set(sl.Len + 3, byte(v))
    sl.Len += 4
}

func append8(s *[]byte, v uint64) {
    sl := checksl(s, 8)
    sl.Set(sl.Len + 0, byte(v >> 56))
    sl.Set(sl.Len + 1, byte(v >> 48))
    sl.Set(sl.Len + 2, byte(v >> 40))
    sl.Set(sl.Len + 3, byte(v >> 32))
    sl.Set(sl.Len + 4, byte(v >> 24))
    sl.Set(sl.Len + 5, byte(v >> 16))
    sl.Set(sl.Len + 6, byte(v >> 8))
    sl.Set(sl.Len + 7, byte(v))
    sl.Len += 8
}

// Static Size Merging Pass: merges constant size instructions as much as possible.
func _PASS_StaticSizeMerging(bb *BasicBlock) {
    for i := bb.Src; i < bb.End; i++ {
        if p := &bb.P[i]; p.Op == OP_size_const {
            for r, j := true, i + 1; r && j < bb.End; i, j = i + 1, j + 1 {
                switch bb.P[j].Op {
                    case _NOP          : break
                    case OP_seek       : break
                    case OP_deref      : break
                    case OP_size_dyn   : break
                    case OP_size_const : p.Iv += bb.P[j].Iv; bb.P[j].Op = _NOP
                    default            : r = false
                }
            }
        }
    }
}

// Seek Merging Pass: merges seeking instructions as much as possible.
func _PASS_SeekMerging(bb *BasicBlock) {
    for i := bb.Src; i < bb.End; i++ {
        if p := &bb.P[i]; p.Op == OP_seek {
            for r, j := true, i + 1; r && j < bb.End; i, j = i + 1, j + 1 {
                switch bb.P[j].Op {
                    case _NOP    : break
                    case OP_seek : p.Iv += bb.P[j].Iv; bb.P[j].Op = _NOP
                    default      : r = false
                }
            }
        }
    }
}

// NOP Elimination Pass: remove instructions that are effectively NOPs (`seek 0`, `size_const 0`)
func _PASS_NopElimination(bb *BasicBlock) {
    for i := bb.Src; i < bb.End; i++ {
        if bb.P[i].Iv == 0 && (bb.P[i].Op == OP_seek || bb.P[i].Op == OP_size_const) {
            bb.P[i].Op = _NOP
        }
    }
}

// Size Check Merging Pass: merges size-checking instructions as much as possible.
func _PASS_SizeCheckMerging(bb *BasicBlock) {
    for i := bb.Src; i < bb.End; i++ {
        if p := &bb.P[i]; p.Op == OP_size_check {
            for r, j := true, i + 1; r && j < bb.End; i, j = i + 1, j + 1 {
                switch bb.P[j].Op {
                    case _NOP          : break
                    case OP_byte       : break
                    case OP_word       : break
                    case OP_long       : break
                    case OP_quad       : break
                    case OP_sint       : break
                    case OP_seek       : break
                    case OP_deref      : break
                    case OP_length     : break
                    case OP_memcpy_be  : break
                    case OP_size_check : p.Iv += bb.P[j].Iv; bb.P[j].Op = _NOP
                    default            : r = false
                }
            }
        }
    }
}

// Literal Merging Pass: merges all consectutive byte, word or long instructions.
func _PASS_LiteralMerging(bb *BasicBlock) {
    p := bb.P
    i := bb.Src

    /* scan every instruction */
    for i < bb.End {
        iv := p[i]
        op := iv.Op

        /* only interested in literal instructions */
        if op < OP_byte || op > OP_quad {
            i++
            continue
        }

        /* byte merging buffer */
        ip := i
        mm := [15]byte{}
        sl := mm[:0:cap(mm)]

        /* scan for consecutive bytes */
        loop: for i < bb.End {
            iv = p[i]
            op = iv.Op

            /* check for OpCode */
            switch op {
                case _NOP     : i++; continue
                case OP_seek  : i++; continue
                case OP_deref : i++; continue
                case OP_byte  : append1(&sl, byte(iv.Iv))
                case OP_word  : append2(&sl, uint16(iv.Iv))
                case OP_long  : append4(&sl, uint32(iv.Iv))
                case OP_quad  : append8(&sl, uint64(iv.Iv))
                default       : break loop
            }

            /* adjust the program counter */
            p[i].Op = _NOP
            i++

            /* commit the buffer if needed */
            for len(sl) >= 8 {
                p[ip] = Instr{Op: OP_quad, Iv: int64(binary.BigEndian.Uint64(sl))}
                sl = sl[8:]
                ip++
            }

            /* move the remaining bytes to the front */
            copy(mm[:], sl)
            sl = mm[:len(sl):cap(mm)]
        }

        /* add the remaining bytes */
        if len(sl) >= 4 { p[ip] = Instr{Op: OP_long, Iv: int64(binary.BigEndian.Uint32(sl))} ; sl = sl[4:]; ip++ }
        if len(sl) >= 2 { p[ip] = Instr{Op: OP_word, Iv: int64(binary.BigEndian.Uint16(sl))} ; sl = sl[2:]; ip++ }
        if len(sl) >= 1 { p[ip] = Instr{Op: OP_byte, Iv: int64(sl[0])}                       ; sl = sl[1:]; ip++ }
    }
}

// Compacting Pass: remove all the placeholder NOP instructions inserted in the previous pass.
func _PASS_Compacting(bb *BasicBlock) {
    var i int
    var j int

    /* copy instructins excluding NOPs */
    for i, j = bb.Src, bb.Src; i < bb.End; i++ {
        if bb.P[i].Op != _NOP {
            bb.P[j] = bb.P[i]
            j++
        }
    }

    /* update basic block end if needed */
    if i != j {
        bb.End = j
    }
}

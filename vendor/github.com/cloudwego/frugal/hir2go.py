#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import string

from typing import List
from typing import Tuple
from typing import Union
from dataclasses import dataclass

@dataclass
class Imm:
    val: int

    def __str__(self) -> str:
        return '%d' % self.val

    def __repr__(self) -> str:
        return '$%d' % self.val

@dataclass
class Reg:
    id: int
    ptr: bool

    def __str__(self) -> str:
        return 'hir.%c%s' % (
            'P' if self.ptr else 'R',
            self.id if self.id >= 0 else 'n' if self.ptr else 'z'
        )

    def __repr__(self) -> str:
        if self.id < 0:
            return '%nil' if self.ptr else '%z'
        else:
            return '%%%c%d' % ('p' if self.ptr else 'r', self.id)

@dataclass
class Mem:
    offs: int
    base: Reg

    def __str__(self) -> str:
        return '%s, %d' % (self.base, self.offs)

    def __repr__(self) -> str:
        return '%d(%s)' % (self.offs, self.base)

@dataclass
class Func:
    addr: int
    name: str

    def __str__(self) -> str:
        return 'hir.LookupCallByName("%s")' % self.name

    def __repr__(self) -> str:
        if not self.name:
            return '*%#x' % self.addr
        else:
            return '*%#x[%s]' % (self.addr, self.name)

@dataclass
class Label:
    name: str

    def __str__(self) -> str:
        return '"%s"' % self.name

    def __repr__(self) -> str:
        return self.name

@dataclass
class RegSeq:
    regs: List[Reg]

    def __repr__(self) -> str:
        return '{%s}' % ', '.join(str(v) for v in self.regs)

    def to_string(self, is_args: bool) -> str:
        return ''.join(
            '.\n%6c%c%-5d(%s)' % (' ', 'A' if is_args else 'R', i, r)
            for i, r in enumerate(self.regs)
        )

@dataclass
class SwitchCase:
    val: Imm
    ref: Label

    def __repr__(self) -> str:
        return 'case %s: %s' % (self.val, self.ref)

@dataclass
class SwitchCaseSeq:
    cases: List[SwitchCase]

    def __str__(self) -> str:
        tab = {
            p.val.val: str(p.ref)
            for p in self.cases
        }
        return '[]string { %s }' % (', '.join(
            tab.get(i, '""')
            for i in range(max(tab) + 1)
        ))

    def __repr__(self) -> str:
        return '{%s}' % ', '.join('%s' % v for v in self.cases)

Operand = Union[
    Imm,
    Reg,
    Mem,
    Func,
    Label,
    RegSeq,
    SwitchCase,
    SwitchCaseSeq,
]

def parse_int(arg: str) -> Tuple[int, str]:
    j = len(arg)
    for i, cc in enumerate(arg):
        if cc not in (string.hexdigits + 'obxOBX+-'):
            j = i
            break
    if not j:
        raise SyntaxError('invalid integer near ' + repr(arg))
    val = arg[:j]
    arg = arg[j:].strip()
    if val.startswith('0b') or val.startswith('0B'):
        return int(val, 2), arg
    elif val.startswith('0x') or val.startswith('0X'):
        return int(val, 16), arg
    elif val.startswith('0'):
        return int(val, 8), arg
    else:
        return int(val), arg

def parse_arg(arg: str) -> Tuple[Operand, str]:
    if arg.startswith('$'):
        ret, arg = parse_int(arg[1:])
        return Imm(ret), arg
    elif arg.startswith('*'):
        name = ''
        addr, arg = parse_int(arg[1:])
        if arg and arg[0] == '[':
            end = arg.find(']', 1)
            if end < 0:
                raise SyntaxError('invalid function name near ' + repr(arg))
            name = arg[1:end]
            arg = arg[end + 1:].strip()
        return Func(addr, name), arg
    elif arg.startswith('%'):
        if arg.startswith('%z'):
            return Reg(-1, False), arg[2:].strip()
        if arg.startswith('%nil'):
            return Reg(-1, True), arg[4:].strip()
        if len(arg) < 3:
            raise SyntaxError('invalid register near ' + repr(arg))
        if arg[1] == 'p':
            ptr = True
        elif arg[1] == 'r':
            ptr = False
        else:
            raise SyntaxError('invalid register near ' + repr(arg))
        j = len(arg)
        for i, cc in enumerate(arg[2:], 2):
            if not cc.isdigit():
                j = i
                break
        if j <= 2:
            raise SyntaxError('invalid register near ' + repr(arg))
        else:
            return Reg(int(arg[2:j]), ptr), arg[j:].strip()
    elif arg.startswith('{'):
        ret = []
        kind = None
        rems = arg[1:].strip()
        while not rems.startswith('}'):
            item, rems = parse_arg(rems)
            ret.append(item)
            if kind is None:
                kind = type(item)
            elif not isinstance(item, kind):
                raise SyntaxError('inconsistent operand list near ' + repr(rems))
            if not rems.startswith('}'):
                if rems.startswith(','):
                    rems = rems[1:].strip()
                else:
                    raise SyntaxError('"," expected between operand list items')
        if not rems.startswith('}'):
            raise SyntaxError('invalid operand list near ' + repr(arg))
        elif kind is SwitchCase:
            return SwitchCaseSeq(ret), rems[1:].strip()
        elif kind is Reg:
            if len(ret) > 8:
                raise SyntaxError('too many registers near ' + repr(arg))
            else:
                return RegSeq(ret), rems[1:].strip()
        else:
            raise SyntaxError('invalid operand list item type near ' + repr(arg))
    elif arg.startswith('case '):
        arg = arg[5:].strip()
        imm, rem = parse_arg(arg)
        if not isinstance(imm, Imm):
            raise SyntaxError('immediate value expected near ' + repr(arg))
        if not rem or rem[0] != ':':
            raise SyntaxError('":" expected near ' + repr(rem))
        rem = rem[1:].strip()
        ref, arg = parse_arg(rem)
        if not isinstance(ref, Label):
            raise SyntaxError('label reference expected near ' + repr(arg))
        else:
            return SwitchCase(imm, ref), arg
    elif arg and arg[0].isdigit():
        off, arg = parse_int(arg)
        if not arg or arg[0] != '(':
            raise SyntaxError('invalid memory address near ' + repr(arg))
        arg = arg[1:].strip()
        base, rem = parse_arg(arg)
        if not rem or rem[0] != ')':
            raise SyntaxError('")" expected near ' + repr(rem))
        elif not isinstance(base, Reg):
            raise SyntaxError('invalid memory address near ' + repr(arg))
        else:
            return Mem(off, base), rem[1:].strip()
    elif arg and arg[0].isidentifier():
        j = len(arg)
        for i, cc in enumerate(arg):
            if cc not in (string.digits + string.ascii_letters + '_'):
                j = i
                break
        return Label(arg[:j]), arg[j:].strip()
    else:
        raise SyntaxError('invalid operand near ' + repr(arg))

def parse_args(args: str) -> List[Operand]:
    ret = []
    args = args.strip()
    while args:
        if ret:
            if args[0] == ',':
                args = args[1:].strip()
            else:
                raise SyntaxError('"," expected between operands')
        val, args = parse_arg(args)
        ret.append(val)
    return ret

with open("decoder.hir") as fp:
    hir = []
    cont = 0
    for line in fp.read().splitlines():
        if not cont:
            hir.append(line)
        else:
            hir[-1] += '\n' + line
        cont += line.count('{')
        cont -= line.count('}')

for line in hir:
    line = line.strip()
    if not line:
        continue
    if line.endswith(':'):
        print('    p.Label ("%s")' % line[:-1])
        continue
    ins, *args = line.split(None, 1)
    ins, args = ins.lower(), sum((parse_args(v.strip()) for v in args), [])
    if ins == 'ret':
        if len(args) != 1 or not isinstance(args[0], RegSeq):
            raise SyntaxError('invalid "ret" operands near ' + repr(line))
        else:
            print('    p.RET   ()' + args[0].to_string(False))
    elif ins in {'ccall', 'gcall', 'icall'}:
        if len(args) != 3 or not isinstance(args[0], Func) or not isinstance(args[1], RegSeq) or not isinstance(args[2], RegSeq):
            raise SyntaxError('invalid "%s" operands near %r' % (ins, line))
        else:
            print('    p.%-6s(%s)%s%s' % (ins.upper(), args[0], args[1].to_string(True), args[2].to_string(False)))
    else:
        print('    p.%-6s(%s)' % (ins.upper(), ', '.join(map(str, args))))

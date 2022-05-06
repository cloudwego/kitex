.PHONY: all clean

C_SRC	 := native/skipping.c
GO_ASM 	 := internal/binary/decoder/native_amd64.s
GO_PROTO := internal/binary/decoder/native_amd64.go

CFLAGS := -mavx
CFLAGS += -mno-bmi
CFLAGS += -mno-avx2
CFLAGS += -mno-red-zone
CFLAGS += -fno-asynchronous-unwind-tables
CFLAGS += -fno-stack-protector
CFLAGS += -fno-exceptions
CFLAGS += -fno-builtin
CFLAGS += -fno-rtti
CFLAGS += -nostdlib
CFLAGS += -O3

all: ${GO_ASM}

clean:
	rm -vf ${GO_ASM} output/*.s

${GO_ASM}: ${C_SRC} ${GO_PROTO}
	mkdir -p output
	clang ${CFLAGS} -S -o output/native.s ${C_SRC}
	python3 tools/asm2asm/asm2asm.py ${GO_ASM} output/native.s
	asmfmt -w ${GO_ASM}

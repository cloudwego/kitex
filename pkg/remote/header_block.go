package remote

import "encoding/binary"

const (
	headerBlockVersion        = 0x34424d54
	headerBlockHeadSize       = 256
	headerBlockEntrySize      = 24
	headerBlockMetadata       = 16
	headerBlockMetadataFields = 18
	headerBlockMetadataSize   = headerBlockMetadataFields + 1
	headerBlockSlots          = 168
	headerBlockIntSlots       = 32
	headerBlockSlotCount      = 44
	headerBlockInitialSize    = 1024
	headerBlockRetainedSize   = 64 << 10
	headerBlockMaxSize        = 1 << 30
	headerBlockMaxEntries     = 65535
)

var headerBlockZero [headerBlockHeadSize]byte

type headerBlock struct {
	data    []byte
	dataEnd int
	count   int
	used    bool
	grown   bool
	pins    *headerBlockPins
}

func (b *headerBlock) recycle() {
	if !b.grown {
		return
	}
	b.grown = false
	b.pins.release()
	if len(b.data) > headerBlockRetainedSize {
		b.data, b.used = nil, false
		return
	}
	b.pins.pin(b.data)
}

func (b *headerBlock) reset() {
	if !b.used {
		return
	}
	copy(b.data, headerBlockZero[:])
	b.dataEnd, b.count, b.used = headerBlockHeadSize, 0, false
}

func (b *headerBlock) ensure(size int) bool {
	if b.data != nil && size <= len(b.data)-b.count*headerBlockEntrySize-b.dataEnd {
		return true
	}
	return b.grow(size)
}

func (b *headerBlock) grow(size int) bool {
	next := headerBlockInitialSize
	if b.data == nil {
		b.dataEnd, b.count = headerBlockHeadSize, 0
		if b.pins == nil {
			b.pins = newHeaderBlockPins()
		}
	} else {
		next = len(b.data) * 2
	}
	entries := b.count * headerBlockEntrySize
	for next < b.dataEnd+entries+size {
		next *= 2
	}
	if next > headerBlockMaxSize {
		return false
	}
	data := make([]byte, next)
	if b.data != nil {
		copy(data, b.data[:b.dataEnd])
		copy(data[next-entries:], b.data[len(b.data)-entries:])
		b.grown = true
	}
	b.pins.pin(data)
	b.data = data
	return true
}

func (b *headerBlock) put(value string) uint32 {
	offset := b.dataEnd
	b.dataEnd += copy(b.data[offset:], value)
	return uint32(offset)
}

func headerBlockStringSlot(key string) int {
	switch key {
	case "RPC_PERSIST_SERVICE_LEVEL":
		return headerBlockIntSlots
	case "RPC_PERSIST_ori_psm":
		return headerBlockIntSlots + 1
	case "RPC_PERSIST_SAMPLE_REQUEST":
		return headerBlockIntSlots + 2
	case "RPC_TRANSIT_secondary-acl-token":
		return headerBlockIntSlots + 3
	case "Rpc-Persist-Lane-P-Did":
		return headerBlockIntSlots + 4
	case "RPC_TRANSIT_EGRESS_SENT_TS":
		return headerBlockIntSlots + 5
	case "sc":
		return headerBlockIntSlots + 6
	case "crrst":
		return headerBlockIntSlots + 7
	case "RPC_PERSIST_crrst":
		return headerBlockIntSlots + 8
	case "K_ProcessAtTime":
		return headerBlockIntSlots + 9
	}
	return -1
}

func (b *headerBlock) append(kind, id uint16, op uint32, key, value string, slot int) (uint32, bool) {
	if b.count == headerBlockMaxEntries || !b.ensure(len(key)+len(value)+headerBlockEntrySize) {
		return 0, false
	}
	b.used = true
	keyAt := b.dataEnd
	if len(key) != 0 {
		b.dataEnd += copy(b.data[keyAt:], key)
	}
	valueAt := b.dataEnd
	if len(value) != 0 {
		b.dataEnd += copy(b.data[valueAt:], value)
	}
	b.count++
	at := len(b.data) - b.count*headerBlockEntrySize
	p := b.data[at : at+headerBlockEntrySize : at+headerBlockEntrySize]
	binary.LittleEndian.PutUint16(p[0:], kind)
	binary.LittleEndian.PutUint16(p[2:], id)
	binary.LittleEndian.PutUint32(p[4:], op)
	binary.LittleEndian.PutUint32(p[8:], uint32(keyAt))
	binary.LittleEndian.PutUint32(p[12:], uint32(len(key)))
	binary.LittleEndian.PutUint32(p[16:], uint32(valueAt))
	binary.LittleEndian.PutUint32(p[20:], uint32(len(value)))
	if slot >= 0 {
		binary.LittleEndian.PutUint16(b.data[headerBlockSlots+2*slot:], uint16(b.count))
	}
	return uint32(valueAt), true
}

func (b *headerBlock) setMetadata(field int, offset uint32, size int) {
	b.used = true
	p := b.data[headerBlockMetadata+8*field:]
	binary.LittleEndian.PutUint32(p[0:], offset)
	binary.LittleEndian.PutUint32(p[4:], uint32(size))
}

func (b *headerBlock) seal() []byte {
	if !b.ensure(0) {
		return nil
	}
	b.used = true
	binary.LittleEndian.PutUint32(b.data[0:], headerBlockVersion)
	binary.LittleEndian.PutUint32(b.data[4:], uint32(len(b.data)))
	binary.LittleEndian.PutUint32(b.data[8:], uint32(b.dataEnd))
	binary.LittleEndian.PutUint32(b.data[12:], uint32(b.count))
	return b.data
}

func (a HeaderAccess) NativeBlock() ([]byte, bool) {
	ti, ok := a.info.(*transInfo)
	if !ok || !ti.primary {
		return nil, false
	}
	b := &ti.store.block
	return b.seal(), b.pins.owned()
}

func (a HeaderAccess) SetNativeMetadata(field int, value string) bool {
	ti, ok := a.info.(*transInfo)
	if !ok || !ti.primary || field < 0 || field >= headerBlockMetadataSize {
		return false
	}
	b := &ti.store.block
	if !b.ensure(len(value)) {
		return false
	}
	b.setMetadata(field, b.put(value), len(value))
	return true
}

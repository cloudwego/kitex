package gonet

import (
	"math/rand"
	"testing"
	"time"
)

func TestSliceBuffer(t *testing.T) {
	chunkSize := 8
	buffer := newSliceBuffer(chunkSize)

	// **测试 1: 基本写入 & 读取**
	data1 := buffer.malloc(4)
	copy(data1, "abcd")

	data2 := buffer.malloc(4)
	copy(data2, "efgh")

	result, err := buffer.next(4)
	if err != nil || string(result) != "abcd" {
		t.Errorf("Expected 'abcd', got %s", string(result))
	}

	peeked, err := buffer.peek(4)
	if err != nil || string(peeked) != "efgh" {
		t.Errorf("Expected 'efgh' in peek, got %s", string(peeked))
	}

	buffer.skip(4)
	//if buffer.readOffset != 8 {
	//	t.Errorf("Expected readOffset = 8, got %d", buffer.readOffset)
	//}

	// **测试 2: 跨 chunk 读写**
	data3 := buffer.malloc(10)
	copy(data3, "ijklmnopqrst")

	result, err = buffer.next(6) // 读取 "ijklmn"
	if err != nil || string(result) != "ijklmn" {
		t.Errorf("Expected 'ijklmn', got %s", string(result))
	}

	result, err = buffer.next(4) // 读取 "opqr"
	if err != nil || string(result) != "opqr" {
		t.Errorf("Expected 'opqr', got %s", string(result))
	}

	// **测试 3: 读取超出范围**
	_, err = buffer.next(5) // "st" 仅剩 2 字节，应该报错
	if err == nil {
		t.Errorf("Expected error when reading beyond available data")
	}

	// **测试 4: 释放未使用数据**
	data4 := buffer.malloc(6)
	copy(data4, "uvwxyz")

	buffer.releaseUnused(2)      // 释放 2 字节
	if buffer.writeOffset != 4 { // "uvwx" 仍然保留
		t.Errorf("Expected writeOffset = 4, got %d", buffer.writeOffset)
	}

	result, err = buffer.next(4)
	if err != nil || string(result) != "uvwx" {
		t.Errorf("Expected 'uvwx', got %s", string(result))
	}

	buffer.release()
}

// 生成随机字符串
func randString(n int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	result := make([]byte, n)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func TestSliceBufferRandomized(t *testing.T) {
	chunkSize := 4096
	buffer := newSliceBuffer(chunkSize)

	expectedData := "" // 维护期望的数据
	offset := 0        // 维护当前读取的位置

	// **1. 随机大小写入**
	for i := 0; i < 1000; i++ {
		size := chunkSize + 1
		data := randString(size)
		buf := buffer.malloc(size)
		if len(buf) != size {
			t.Errorf("buffer size %d not equal to size %d", len(buf), size)
		}
		copy(buf, data)
		expectedData += data // 记录期望值
	}

	s, err := buffer.peek(len(expectedData))
	if err != nil {
		t.Fatal(err)
	}
	if expectedData != string(s) {
		t.Fatalf("Expected len=%d, Got: len=%d", len(expectedData), len(s))
	}

	// **2. 随机 `next`、`peek`、`skip`**
	for i := 0; i < 1000; i++ {
		op := rand.Intn(3)                 // 0: next, 1: peek, 2: skip
		size := rand.Intn(2*chunkSize) + 1 // 1~5 字节

		if offset+size > len(expectedData) {
			size = len(expectedData) - offset // 避免超出范围
		}

		switch op {
		case 0:
			data, err := buffer.next(size)
			if err != nil {
				t.Fatal(err)
			}
			expected := expectedData[offset : offset+size]
			if expected != string(data) {
				readSize := offset + size
				t.Fatalf("read size=%d, Expected: %s, Got: %s", readSize, expected, data)
			}
			offset += size
		case 1:
			data, err := buffer.peek(size)
			if err != nil {
				t.Fatal(err)
			}
			expected := expectedData[offset : offset+size]
			if expected != string(data) {
				t.Fatalf("Expected: %s, Got: %s", expected, data)
			}
		case 2:
			err := buffer.skip(size)
			if err != nil {
				t.Fatalf("[skip] err=%v", err)
			}
			offset += size
		}
	}

	// **3. 释放未使用数据并写入**
	releaseSize := rand.Intn(5) + 1
	buffer.releaseUnused(releaseSize)

	// **4. 超界读取**
	overReadSize := buffer.readableLen() + 5
	data, err := buffer.next(overReadSize)
	if err == nil {
		t.Fatalf("Expected error when over-reading, but got: %s", data)
	}
}

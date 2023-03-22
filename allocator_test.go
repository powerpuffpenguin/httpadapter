package httpadapter_test

import "github.com/powerpuffpenguin/easygo/bytes"

var clientAllocator = bytes.NewPool(
	[]bytes.BlockAllocator{
		bytes.NewAllocatorPool(32, true, 1000),
		bytes.NewAllocatorPool(128, true, 1000),
		bytes.NewAllocatorPool(256, true, 1000),
		bytes.NewAllocatorPool(512, true, 1000),
		bytes.NewAllocatorPool(1024, true, 1000),
		bytes.NewAllocatorPool(1024*2, true, 1000),
		bytes.NewAllocatorPool(1024*4, true, 1000),
		bytes.NewAllocatorPool(1024*8, true, 1000),
		bytes.NewAllocatorPool(1024*16, true, 1000),
		bytes.NewAllocatorPool(1024*32, true, 1000),
	},
	bytes.PoolBeforeGet(func(size int) []byte {
		if size < 1 || size > 1024*32 {
			return make([]byte, size)
		}
		return nil
	}),
	bytes.PoolBeforePut(func(b []byte) bool {
		size := len(b)
		return size < 1 || size > 1024*32
	}),
)
var defaultAllocator = clientAllocator

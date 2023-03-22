package memory

type Buffer struct {
	// The actual data
	Data []byte
	// The allocator returns the slice, if nil then not from the allocator
	Buffer []byte
}

// This is a memory allocation and release interface.
// Although go comes with a memory pool,
// a custom memory pool can improve program efficiency and reduce fragmentation at some point.
type BytesAllocator interface {
	// Memory is no longer in use release it
	Put(b []byte)
	// Get a block of memory
	Get(size int) []byte
}
type BufferAllocator struct {
	Allocator BytesAllocator
}

func (a BufferAllocator) Put(b Buffer) {
	if a.Allocator != nil && b.Buffer != nil {
		a.Allocator.Put(b.Buffer)
	}
}
func (a BufferAllocator) Get(size int) Buffer {
	var b []byte
	if a.Allocator == nil {
		b = make([]byte, size)
	} else {
		b = a.Allocator.Get(size)
	}
	return Buffer{
		Data:   b,
		Buffer: b,
	}
}

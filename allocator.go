package httpadapter

import "github.com/powerpuffpenguin/httpadapter/internal/memory"

type Allocator memory.BytesAllocator

var defaultAllocator memory.BufferAllocator

#include <stddef.h>
#include <stdalign.h>

// 1kb heap for now
#define HEAP_SIZE_BYTES (1 << 10)

#define ALIGNMENT alignof(max_align_t)
// round up to nearest power of ALIGNMENT
#define ALIGN_UP(n) (((n) + (ALIGNMENT - 1)) & ~(size_t)(ALIGNMENT - 1))

void* acc_alloc(size_t req) {
    static size_t i = 0;
    static alignas(max_align_t) char heap[HEAP_SIZE_BYTES];

    if(req > HEAP_SIZE_BYTES) {
        return 0;
    }
    req = ALIGN_UP(req);

    // check if heap is large enough to accomodate requested size.
    if(req > HEAP_SIZE_BYTES - i) {
        return 0;
    }
    // bump heap head
    i += req;
    // pointer to new allocation
    return (i - req) + heap;
}

#ifndef DIFFMANTIC_FLAT_BRIDGE_H
#define DIFFMANTIC_FLAT_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

#define FLAT_NODE_NAMED   (1 << 0)
#define FLAT_NODE_ERROR   (1 << 1)
#define FLAT_NODE_MISSING (1 << 2)

typedef struct {
    uint16_t type_id;
    uint16_t flags;
    uint32_t start_byte;
    uint32_t end_byte;
    uint32_t start_row;
    uint32_t start_col;
    uint32_t end_row;
    uint32_t end_col;
    uint32_t parent_idx;       /* 0xFFFFFFFF for root */
    uint32_t first_child_idx;  /* 0xFFFFFFFF if leaf */
    uint32_t next_sibling_idx; /* 0xFFFFFFFF if last child */
    uint32_t child_count;
} FlatNode;

typedef struct {
    const FlatNode* nodes_ptr;
    size_t node_count;
    int32_t error_code;
} FlatASTResult;

#ifdef __cplusplus
extern "C" {
#endif

struct TSLanguage;
const struct TSLanguage* diffmantic_get_native_language(const char* name);

FlatASTResult parse_to_flat_ast(
    const uint8_t* src_ptr,
    size_t src_len,
    const void* ts_language_ptr
);

void free_flat_ast(FlatASTResult result);

#ifdef __cplusplus
}
#endif

#endif /* DIFFMANTIC_FLAT_BRIDGE_H */

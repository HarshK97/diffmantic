#include "bridge.h"
#include <tree_sitter/api.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    FlatNode* items;
    size_t len;
    size_t cap;
} FlatNodeList;

static int list_init(FlatNodeList* list, size_t initial_cap) {
    list->len = 0;
    list->cap = initial_cap > 0 ? initial_cap : 256;
    list->items = (FlatNode*)malloc(list->cap * sizeof(FlatNode));
    return list->items != NULL ? 0 : -1;
}

static int list_append(FlatNodeList* list, FlatNode node, uint32_t* out_idx) {
    if (list->len >= list->cap) {
        size_t new_cap = list->cap * 2;
        FlatNode* new_items = (FlatNode*)realloc(list->items, new_cap * sizeof(FlatNode));
        if (!new_items) return -1;
        list->items = new_items;
        list->cap = new_cap;
    }
    uint32_t idx = (uint32_t)list->len;
    list->items[idx] = node;
    list->len++;
    if (out_idx) *out_idx = idx;
    return 0;
}

static int flatten_subtree(TSTreeCursor* cursor, FlatNodeList* list, uint32_t parent_idx, uint32_t* out_my_idx) {
    TSNode node = ts_tree_cursor_current_node(cursor);
    TSPoint start_pt = ts_node_start_point(node);
    TSPoint end_pt = ts_node_end_point(node);

    uint16_t flags = 0;
    if (ts_node_is_named(node)) flags |= FLAT_NODE_NAMED;
    if (ts_node_is_error(node)) flags |= FLAT_NODE_ERROR;
    if (ts_node_is_missing(node)) flags |= FLAT_NODE_MISSING;

    FlatNode fn;
    fn.type_id = ts_node_symbol(node);
    fn.flags = flags;
    fn.start_byte = ts_node_start_byte(node);
    fn.end_byte = ts_node_end_byte(node);
    fn.start_row = start_pt.row;
    fn.start_col = start_pt.column;
    fn.end_row = end_pt.row;
    fn.end_col = end_pt.column;
    fn.parent_idx = parent_idx;
    fn.first_child_idx = 0xFFFFFFFF;
    fn.next_sibling_idx = 0xFFFFFFFF;
    fn.child_count = ts_node_child_count(node);

    uint32_t my_idx = 0;
    if (list_append(list, fn, &my_idx) != 0) return -1;
    if (out_my_idx) *out_my_idx = my_idx;

    if (ts_tree_cursor_goto_first_child(cursor)) {
        int first_child = 1;
        uint32_t prev_child_idx = 0xFFFFFFFF;

        while (1) {
            uint32_t child_idx = 0;
            if (flatten_subtree(cursor, list, my_idx, &child_idx) != 0) {
                return -1;
            }

            if (first_child) {
                list->items[my_idx].first_child_idx = child_idx;
                first_child = 0;
            } else {
                list->items[prev_child_idx].next_sibling_idx = child_idx;
            }
            prev_child_idx = child_idx;

            if (!ts_tree_cursor_goto_next_sibling(cursor)) break;
        }

        ts_tree_cursor_goto_parent(cursor);
    }

    return 0;
}

FlatASTResult parse_to_flat_ast(
    const uint8_t* src_ptr,
    size_t src_len,
    const void* ts_language_ptr
) {
    if (!ts_language_ptr) {
        return (FlatASTResult){NULL, 0, -1};
    }

    const char* text = src_ptr ? (const char*)src_ptr : "";
    const TSLanguage* lang = (const TSLanguage*)ts_language_ptr;
    TSParser* parser = ts_parser_new();
    if (!parser) return (FlatASTResult){NULL, 0, -2};

    if (!ts_parser_set_language(parser, lang)) {
        ts_parser_delete(parser);
        return (FlatASTResult){NULL, 0, -3};
    }

    TSTree* tree = ts_parser_parse_string(parser, NULL, text, (uint32_t)src_len);
    ts_parser_delete(parser);
    if (!tree) return (FlatASTResult){NULL, 0, -4};

    FlatNodeList list;
    if (list_init(&list, src_len > 2048 ? src_len / 16 : 256) != 0) {
        ts_tree_delete(tree);
        return (FlatASTResult){NULL, 0, -5};
    }

    TSTreeCursor cursor = ts_tree_cursor_new(ts_tree_root_node(tree));
    if (flatten_subtree(&cursor, &list, 0xFFFFFFFF, NULL) != 0) {
        ts_tree_cursor_delete(&cursor);
        ts_tree_delete(tree);
        free(list.items);
        return (FlatASTResult){NULL, 0, -6};
    }

    ts_tree_cursor_delete(&cursor);
    ts_tree_delete(tree); /* Release native syntax tree now that nodes are copied into the flat buffer. */

    return (FlatASTResult){
        .nodes_ptr = list.items,
        .node_count = list.len,
        .error_code = 0,
    };
}

void free_flat_ast(FlatASTResult result) {
    if (result.nodes_ptr) {
        free((void*)result.nodes_ptr);
    }
}

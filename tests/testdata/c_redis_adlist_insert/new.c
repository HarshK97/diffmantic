#include <stdlib.h>

typedef struct listNode {
    struct listNode *prev;
    struct listNode *next;
    void *value;
} listNode;

typedef struct list {
    listNode *head;
    listNode *tail;
    unsigned long len;
} list;

list *listCreate(void) {
    list *l = malloc(sizeof(*l));
    if(!l) return NULL;
    l->head = l->tail = NULL;
    l->len = 0;
    return l;
}

list *listInsertNode(list *list, listNode *old_node, void *value, int after) {
    if(!list || !old_node) return NULL;
    listNode *node = malloc(sizeof(*node));
    if(!node) return NULL;
    node->value = value;

    if(after) {
        node->prev = old_node;
        node->next = old_node->next;
        if(list->tail == old_node) {
            list->tail = node;
        } else {
            node->next->prev = node;
        }
        old_node->next = node;
    } else {
        node->next = old_node;
        node->prev = old_node->prev;
        if(list->head == old_node) {
            list->head = node;
        } else {
            node->prev->next = node;
        }
        old_node->prev = node;
    }
    list->len++;
    return list;
}

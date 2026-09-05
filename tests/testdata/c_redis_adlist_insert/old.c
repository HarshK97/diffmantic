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
    listNode *node = malloc(sizeof(*node));
    if(!node) return NULL;
    node->value = value;

    if(after) {
        node->prev = old_node;
        node->next = old_node->next;
        if(list->tail == old_node) {
            list->tail = node;
        }
    } else {
        node->next = old_node;
        node->prev = old_node->prev;
        if(list->head == old_node) {
            list->head = node;
        }
    }
    if(node->prev != NULL) node->prev->next = node;
    if(node->next != NULL) node->next->prev = node;
    list->len++;
    return list;
}

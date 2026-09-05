#include <stdlib.h>
#include <stdint.h>

typedef struct dictEntry {
    void *key;
    void *val;
    struct dictEntry *next;
} dictEntry;

typedef struct dictht {
    dictEntry **table;
    unsigned long size;
    unsigned long sizemask;
    unsigned long used;
} dictht;

typedef struct dict {
    dictht ht[2];
    long rehashidx;
} dict;

int dictResize(dict *d) {
    unsigned long minimal;

    if(!d || d->rehashidx != -1) return -1;
    minimal = d->ht[0].used;
    if(minimal < 4)
        minimal = 4;
    if(d->ht[0].size == minimal)
        return 0;
    return 1;
}

int dictIsRehashing(dict *d) {
    return d && d->rehashidx != -1;
}

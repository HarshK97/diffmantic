#include <stdlib.h>
#include <string.h>

typedef struct sdshdr {
    unsigned int len;
    unsigned int free;
    char buf[];
} sdshdr;

char *sdsnewlen(const void *init, size_t initlen) {
    sdshdr *sh = malloc(sizeof(sdshdr) + initlen + 1);
    if(!sh) return NULL;
    sh->len = (unsigned int)initlen;
    sh->free = 0;
    if(initlen && init)
        memcpy(sh->buf, init, initlen);
    sh->buf[initlen] = '\0';
    return sh->buf;
}

char *sdsMakeRoomFor(char *s, size_t addlen) {
    sdshdr *sh = (sdshdr*)(s - sizeof(sdshdr));
    if(sh->free >= addlen) return s;

    size_t len = sh->len;
    size_t newlen = len + addlen;
    if(newlen < 1024*1024) {
        newlen = (newlen < 32) ? 32 : newlen * 2;
    } else {
        newlen += 1024*1024;
    }

    sdshdr *newsh = realloc(sh, sizeof(sdshdr) + newlen + 1);
    if(!newsh) return NULL;

    newsh->free = (unsigned int)(newlen - newsh->len);
    return newsh->buf;
}

void sdsfree(char *s) {
    if(!s) return;
    free(s - sizeof(sdshdr));
}

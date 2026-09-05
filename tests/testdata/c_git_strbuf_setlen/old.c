#include <stdlib.h>
#include <string.h>

struct strbuf {
    size_t alloc;
    size_t len;
    char *buf;
};

void strbuf_init(struct strbuf *sb, size_t alloc) {
    sb->alloc = alloc;
    sb->len = 0;
    if(alloc) {
        sb->buf = malloc(alloc);
        if(sb->buf)
            sb->buf[0] = '\0';
    } else {
        sb->buf = NULL;
    }
}

void strbuf_grow(struct strbuf *sb, size_t extra) {
    if(sb->len + extra + 1 <= sb->alloc)
        return;

    size_t new_alloc = (sb->alloc + extra + 16) * 3 / 2;
    char *new_buf = realloc(sb->buf, new_alloc);
    if(new_buf) {
        sb->buf = new_buf;
        sb->alloc = new_alloc;
    }
}

void strbuf_setlen(struct strbuf *sb, size_t len) {
    if(len > sb->alloc)
        return;
    sb->len = len;
    if(sb->buf)
        sb->buf[len] = '\0';
}

void strbuf_release(struct strbuf *sb) {
    free(sb->buf);
    sb->buf = NULL;
    sb->alloc = 0;
    sb->len = 0;
}

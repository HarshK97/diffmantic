#include <stdint.h>
#include <stdlib.h>

typedef struct uv_loop_s uv_loop_t;
typedef struct uv_timer_s uv_timer_t;
typedef void (*uv_timer_cb)(uv_timer_t*);

struct uv_timer_s {
    uv_loop_t* loop;
    uv_timer_cb timer_cb;
    uint64_t timeout;
    uint64_t repeat;
    int active;
};

int uv_timer_init(uv_loop_t* loop, uv_timer_t* handle) {
    if(!handle) return -1;
    handle->loop = loop;
    handle->timer_cb = NULL;
    handle->timeout = 0;
    handle->repeat = 0;
    handle->active = 0;
    return 0;
}

int uv_timer_start(uv_timer_t* handle, uv_timer_cb cb, uint64_t timeout, uint64_t repeat) {
    if(!handle || !cb) return -1;

    handle->timer_cb = cb;
    handle->timeout = (timeout == (uint64_t)-1) ? 0 : timeout;
    handle->repeat = repeat;
    handle->active = 1;
    return 0;
}

int uv_timer_stop(uv_timer_t* handle) {
    if(!handle) return -1;
    if(!handle->active) return 0;
    handle->active = 0;
    return 0;
}

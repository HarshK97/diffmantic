#include <stdlib.h>
#include <string.h>

typedef struct StrAccum StrAccum;
struct StrAccum {
    char *zText;
    int nChar;
    int nAlloc;
    int mxAlloc;
    int accError;
};

void sqlite3StrAccumInit(StrAccum *p, char *zBase, int n, int mx) {
    p->zText = zBase;
    p->nChar = 0;
    p->nAlloc = n;
    p->mxAlloc = mx;
    p->accError = 0;
}

void sqlite3StrAccumAppend(StrAccum *p, const char *z, int N) {
    if(p->accError || N <= 0) return;

    if(p->nChar + N + 1 > p->nAlloc) {
        int newAlloc = (p->nAlloc ? p->nAlloc * 2 : 64) + N;
        if(newAlloc > p->mxAlloc) {
            newAlloc = p->mxAlloc;
            if(p->nChar + N + 1 > newAlloc) {
                p->accError = 1;
                return;
            }
        }
        char *zNew = realloc(p->zText, newAlloc);
        if(!zNew) {
            p->accError = 1;
            return;
        }
        p->zText = zNew;
        p->nAlloc = newAlloc;
    }

    memcpy(&p->zText[p->nChar], z, N);
    p->nChar += N;
    p->zText[p->nChar] = '\0';
}

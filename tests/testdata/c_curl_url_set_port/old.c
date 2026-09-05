#include <stdlib.h>
#include <string.h>
#include <ctype.h>

struct Curl_URL {
    char *scheme;
    char *host;
    char *port;
    char *path;
    int port_num;
};

int Curl_set_port(struct Curl_URL *u, const char *port_str) {
    if(!u || !port_str)
        return -1;

    for(size_t i = 0; port_str[i]; i++) {
        if(!isdigit((unsigned char)port_str[i]))
            return -1;
    }

    int port = atoi(port_str);
    if(port <= 0 || port > 65535)
        return -1;

    free(u->port);
    u->port = strdup(port_str);
    if(!u->port)
        return -2;

    u->port_num = port;
    return 0;
}

int Curl_clear_port(struct Curl_URL *u) {
    if(!u)
        return -1;

    free(u->port);
    u->port = NULL;
    u->port_num = 0;
    return 0;
}

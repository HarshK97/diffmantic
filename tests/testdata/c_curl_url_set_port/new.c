#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <limits.h>

struct Curl_URL {
    char *scheme;
    char *host;
    char *port;
    char *path;
    int port_num;
};

int Curl_set_port(struct Curl_URL *u, const char *port_str) {
    if(!u || !port_str || !*port_str)
        return -1;

    char *endptr = NULL;
    long port = strtol(port_str, &endptr, 10);
    if(*endptr != '\0')
        return -1;

    if(port <= 0 || port > 65535)
        return -1;

    char *new_port = strdup(port_str);
    if(!new_port)
        return -2;

    free(u->port);
    u->port = new_port;
    u->port_num = (int)port;
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

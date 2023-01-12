#ifndef NET_H
#define NET_H

#include <arpa/inet.h>
#include <errno.h>
#include <netdb.h>
#include <netinet/in.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <unistd.h>

const char* kUdpPort = "8080";  // port we sendto and recvfrom
const int kBufLen = 128;        // max number of bytes we can recv of send

void panic(const char* format, ...) {
  va_list arglist;
  va_start(arglist, format);
  vfprintf(stderr, strcat("panic: ", format), arglist);
  va_end(arglist);
  exit(EXIT_FAILURE);
}

#endif
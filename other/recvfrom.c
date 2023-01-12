/*
 * Binds a socket to port 8080 and invokes recvfrom in a loop.
 * The peer address is filled in automatically based on the sender.
 * So there can be multiple senders.
 */
#include "net.h"

int main(void) {
  printf("starting recvfrom...\n");

  struct addrinfo hints;
  memset(&hints, 0, sizeof(hints));
  hints.ai_family = AF_INET;  // IPv4
  hints.ai_socktype = SOCK_DGRAM;
  hints.ai_flags = AI_PASSIVE;

  struct addrinfo* addrs;
  int ret = getaddrinfo(NULL, kUdpPort, &hints, &addrs);
  if (ret != 0) {
    panic("getaddrinfo err=%s", gai_strerror(ret));
  }

  int sockfd = -1;
  for (struct addrinfo* addr = addrs; addr != NULL; addr = addrs->ai_next) {
    sockfd = socket(addr->ai_family, addr->ai_socktype, addr->ai_protocol);
    if (sockfd < 0) {
      continue;
    }

    if (bind(sockfd, addr->ai_addr, addr->ai_addrlen) == -1) {
      close(sockfd);
      sockfd = -1;
      continue;
    }

    break;  // we have bound a socket
  }
  if (sockfd < 0) {
    panic("socket err=%d", sockfd);
  }

  freeaddrinfo(addrs);

  char buf[kBufLen];  // buffer to read what we receive

  char str_peer_addr_buf[128];

  struct sockaddr_storage
      peer_addr;  // filled in by recvfrom, so we need to allocate
  socklen_t peer_addr_len = sizeof(peer_addr);
  for (;;) {
    int n = recvfrom(sockfd, buf, kBufLen - 1 /* reserve one for \0 */, 0,
                     (struct sockaddr*)&peer_addr, &peer_addr_len);
    if (n < 0) {
      panic("recvfrom err=%d", n);
    } else {
      void* raw_peer_addr;
      if (peer_addr.ss_family == AF_INET) {
        // IPv4
        raw_peer_addr = &(((struct sockaddr_in*)&peer_addr)->sin_addr);
      } else {
        // IPv6
        raw_peer_addr = &(((struct sockaddr_in6*)&peer_addr)->sin6_addr);
      }

      const char* str_peer_addr =
          addr_to_str(str_peer_addr_buf, sizeof(str_peer_addr_buf),
                      (struct sockaddr*)&peer_addr);
      if (str_peer_addr == NULL) {
        panic("could not parse peer address");
      } else {
        buf[n] = '\0';
        printf("recv packet from %s of size %d bytes data=%s\n", str_peer_addr,
               n, buf);
      }
    }
  }

  return EXIT_SUCCESS;
}
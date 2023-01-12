/* udp_sendto.c
 *
 * Binds a socket to port 8080 and invokes recvfrom in a loop.
 * The peer address is filled in automatically based on the sender.
 * So there can be multiple senders.
 */
#include "net.h"

const char* kSend = "hello";

int main(int argc, char* argv[]) {
  printf("starting sendto...\n");

  char server_node[128];
  if (argc == 1) {
    strcpy(server_node, "127.0.0.1");
  } else if (argc == 2) {
    strcpy(server_node, argv[1]);
  } else {
    panic(
        "usage: udp_sendto <address_to_sendto>; arg is optional, localhost by "
        "default.");
  }

  struct addrinfo hints;
  memset(&hints, 0, sizeof(hints));
  hints.ai_family = AF_INET;

  struct addrinfo* addrs;
  int ret = getaddrinfo(server_node, kUdpPort, &hints, &addrs);
  if (ret != 0) {
    panic("getaddrinfo err=%s", gai_strerror(ret));
  }

  int sockfd = -1;
  struct addrinfo* peer_addr;
  for (peer_addr = addrs; peer_addr != NULL; peer_addr = peer_addr->ai_next) {
    sockfd = socket(peer_addr->ai_family, peer_addr->ai_socktype,
                    peer_addr->ai_protocol);
    if (sockfd < 0) {
      continue;
    }

    // no need to bind it ourselves, the kernel will assign a port
    // automatically

    break;  // socket created and bound
  }
  if (sockfd < 0) {
    panic("socket err=%d", sockfd);
  }

  for (int i = 0; i < 10; ++i) {
    int n = sendto(sockfd, kSend, sizeof(kSend), 0, peer_addr->ai_addr,
                   peer_addr->ai_addrlen);
    if (n < 0) {
      panic("sendto err=%d", n);
    } else {
      printf("sent %s successfully\n", kSend);
    }
  }

  freeaddrinfo(addrs);  // only here otherwise we invalidate peer_addr

  return EXIT_SUCCESS;
}
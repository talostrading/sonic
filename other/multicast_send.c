#include "net.h"

/*
multicast range: 224.0.0.0 to 239.255.255.255
some address are routable: https://en.wikipedia.org/wiki/Multicast_address
*/

int main(void) {
  // we do not bind the socket to some address we like here, we just let sendto
  // do that for us on the first call.
  int sockfd = socket(AF_INET, SOCK_DGRAM, 0);
  if (sockfd < 0) panic("socket");

  struct sockaddr_in multicast_addr;
  memset(&multicast_addr, 0, sizeof(multicast_addr));
  multicast_addr.sin_family = AF_INET;
  multicast_addr.sin_addr.s_addr = inet_addr(MULTICAST_ADDR);
  multicast_addr.sin_port = htons(MULTICAST_PORT);

  char addr_str_buf[128];
  const char* addr_str =
      addr_to_str(addr_str_buf, 128, (struct sockaddr*)&multicast_addr);
  if (addr_str == NULL) panic("addr_to_str");
  printf("setup mutlicast group %s:%d\n", addr_str, MULTICAST_PORT);

  const char* msg = "hello multicast";

  for (;;) {
    int ret = sendto(sockfd, msg, strlen(msg), 0,
                     (struct sockaddr*)&multicast_addr, sizeof(multicast_addr));
    if (ret < 0) panic("sendto");
    printf("sent %s\n", msg);
    sleep(1);
  }

  return EXIT_SUCCESS;
}

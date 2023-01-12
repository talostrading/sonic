/*
 * Broadcast messages to all hosts on the network.
 * Receivers should bind to INADDR_ANY (which is usually 0.0.0.0) and port
 * kUdpPort from net.h.
 */
#include "net.h"

const char* kSend = "hello";

int main(int argc, char* argv[]) {
  if (argc != 1) {
    panic("usage: broadcast; it sends to 255.255.255.255");
  }

  int sockfd = socket(AF_INET, SOCK_DGRAM, 0);
  if (sockfd < 0) panic("socket");

  int enable = 1;
  // SO_BROADCAST is required before sending any datagrams to the default
  // broadcast (255.255.255.255) or the subnet broadcast address (192.168.x.255
  // if subnet_id is 24 bits for example)
  int ret =
      setsockopt(sockfd, SOL_SOCKET, SO_BROADCAST, &enable, sizeof(enable));
  if (ret != 0) panic("setsockopt");

  struct sockaddr_in broadcast_addr;
  int broadcast_addr_len = sizeof(struct sockaddr_in);
  memset((void*)&broadcast_addr, 0, broadcast_addr_len);
  broadcast_addr.sin_family = AF_INET;
  // INADDR_BROADCAST trickles down to the ethernet layer. If an ethernet card
  // sees some packets destined to 0xff..ff then it starts parsing them
  broadcast_addr.sin_addr.s_addr = htonl(INADDR_BROADCAST);
  broadcast_addr.sin_port = htons(atoi(kUdpPort));

  char addr_str_buf[128];
  const char* addr_str = addr_to_str(addr_str_buf, sizeof(addr_str_buf),
                                     (struct sockaddr*)&broadcast_addr);
  printf("broadcasting to %s:%s\n", addr_str, kUdpPort);

  ret = sendall(sockfd, kSend, strlen(kSend), 0,
                (struct sockaddr*)&broadcast_addr, broadcast_addr_len);
  if (ret != 0) {
    panic("sendall");
  }

  return EXIT_SUCCESS;
}
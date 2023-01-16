#include "net.h"

// multicast address should appear in the routing table
/*
netstat -nr # check the table
sudo route -nv add -net 224.0.42.42 -interface en0 # if not in table add it

# example tcpdump client
sudo tcpdump -ni en0 host 224.0.42.42

# example ping server
ping -t 1 -c 100 224.0.42.42 # if all well then tcpdump should show ICMP traffic

*/

/*
a few notes on addresses:
sockaddr is generic, shared by different types of sockets
for TCP/IP this becomes sockaddr_in (IPv4) or sockaddr_in6(IPv6) or sockaddr_un
for unix domain sockets.

*/

int main(void) {
  int port = 9090;
  struct sockaddr_in addr;
  memset(&addr, 0, sizeof(addr));
  addr.sin_addr.s_addr = htonl(INADDR_ANY);
  addr.sin_port = htons(port);
  addr.sin_family = AF_INET;
  int addrlen = sizeof(addr);

  int sockfd = socket(AF_INET, SOCK_DGRAM, 0);
  if (sockfd < 0) panic("socket");

  int ret = bind(sockfd, (struct sockaddr*)&addr, sizeof(addr));
  if (ret < 0) panic("bind");

  char addr_str_buf[128];
  const char* addr_str =
      addr_to_str(addr_str_buf, 128, (struct sockaddr*)&addr);
  if (addr_str == NULL) panic("addr_to_str");

  printf("bound to %s:%d\n", addr_str, port);

  struct ip_mreqn mreq;  // for IPv4
  mreq.imr_multiaddr.s_addr = inet_addr(MULTICAST_ADDR);
  mreq.imr_address.s_addr = htonl(INADDR_ANY);
  mreq.imr_ifindex = 0;  // let the kernel decide the interface
  ret = setsockopt(sockfd, IPPROTO_IP, IP_ADD_MEMBERSHIP, (char*)&mreq,
                   sizeof(mreq));
  if (ret < 0) panic("setsockopt");

  get_sock_info(sockfd);

  char buf[128];
  for (;;) {
    int n = recvfrom(sockfd, buf, sizeof(buf), 0, (struct sockaddr*)&addr,
                     &addrlen);
    if (n < 0) panic("recvfrom");
    buf[n] = '\0';
    printf("received %s\n", buf);
  }
}
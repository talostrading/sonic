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
  // this is the address and port from which the socket receives data
  // i.e. the peer is a udp sender bound to a group address
  //
  // this has a filtering role: the socket will only receive datagrams sent to
  // this multicast address and port no matter what groups are joined by the
  // socket on the IP level.
  //
  // if you wanna receive all datagrams sent to the port, bind to INADDR_ANY
  struct sockaddr_in addr;  // address and port from which we receive data
  memset(&addr, 0, sizeof(addr));
  addr.sin_addr.s_addr = INADDR_ANY;
  addr.sin_port = htons(MULTICAST_PORT);
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

  printf("bound to %s:%d\n", addr_str, MULTICAST_PORT);

  // this is needed by the interface. Without it, the interface will discard
  // stuff that doesn't match its MAC address
  // but the ethernet layer has a specific set of mac addresses for multicast
  // (1-1 deterministic mapping from IP to link layer)
  // so for multicast, you need to tell the interface to not discard messages
  // destined to the multicast address, which of course doesn't match the MAC
  // address
  //
  // the interface doesn't care about the port number. Port filtering is done on
  // the TCP/IP layer
  struct ip_mreqn mreq;  // for IPv4
  mreq.imr_multiaddr.s_addr = inet_addr(MULTICAST_ADDR);
  mreq.imr_address.s_addr = addr.sin_addr.s_addr;
  mreq.imr_ifindex = 0;  // let the kernel decide the interface
  ret = setsockopt(sockfd, IPPROTO_IP, IP_ADD_MEMBERSHIP, (char*)&mreq,
                   sizeof(mreq));

  // even if everything is good and happy you might have not actually joined the
  // multicast group check the memberships with netstat -g

  if (ret < 0) {
    if (errno == EBADF) {
      panic("setsockopt EBADF");
    } else if (errno == EFAULT) {
      panic("setsockopt EFAULT");
    } else if (errno == EINVAL) {
      panic("setsockopt EINVAL");
    } else if (errno == ENOPROTOOPT) {
      panic("setsockopt ENOPROTOOPT");
    } else if (errno == ENOTSOCK) {
      panic("setsockopt ENOTSOCK");
    }
  }

  // get_sock_info(sockfd);

  char buf[128];
  for (;;) {
    int n = recvfrom(sockfd, buf, sizeof(buf), 0, (struct sockaddr*)&addr,
                     &addrlen);
    if (n < 0) panic("recvfrom");
    buf[n] = '\0';
    logline("received %s\n", buf);
  }
}
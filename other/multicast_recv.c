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

  // for UDP multicast we can have multiple sockets listening to the same
  // multicast group so they need to be bound to the same multicast port so we
  // need SO_REUSEADDR.
  //
  // there is a trick here: INADDR_ANY is a unicast address. So what we actually
  // need is unicast SO_REUSEPORT behaviour but for a multicast bound socket.
  // Hence we use SO_REUSEPORT.
  //
  // if the addr would not be INADDR_ANY, we could use SO_REUSEADDR.
  //
  // summary of SO_REUSEADDR for unicast (or point to point hence also TCP)
  // addresses = has an effect on wildcard addresses (as in I can bind some
  // process to 0.0.0.0:8080 and another to 192.168.0.0:8080 without getting
  // EADDRINUSE) and on TCP sockets in TIME_WAIT state (I can reuse a TIME_WAIT
  // socket). It only care about the state of the current socket, not about the
  // state of the other sockets that are already bound.
  //
  // summary of SO_REUSEPORT for ... (same as above) ... = bind an arbitrary
  // amount of sockets to exactly the same source address and port as long as
  // all prior bound sockets also had SO_REUSEPORT set before they were bound.
  // So for TIME_WAIT TCP sockets. Either set SO_REUSEADDR on the new socket or
  // SO_REUSEPORT on both sockets.j
  int enabled = 1;
  int ret =
      setsockopt(sockfd, SOL_SOCKET, SO_REUSEPORT, &enabled, sizeof(enabled));
  if (ret < 0) panic("setsockopt SO_REUSEPORT");

  // discard all the above and also do SO_REUSEADDR cause you never know
  ret = setsockopt(sockfd, SOL_SOCKET, SO_REUSEADDR, &enabled, sizeof(enabled));
  if (ret < 0) panic("setsockopt SO_REUSEADDR");

  ret = bind(sockfd, (struct sockaddr*)&addr, sizeof(addr));
  if (ret < 0) {
    if (errno == EADDRINUSE) {
      panic("bind EADDRINUSE");
    }
    panic("bind");
  }

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
  struct sockaddr_in sender_addr;
  socklen_t sender_addrlen = sizeof(sender_addr);
  char sender_addr_buf[128];
  for (;;) {
    int n = recvfrom(sockfd, buf, sizeof(buf), 0,
                     (struct sockaddr*)&sender_addr, &sender_addrlen);
    if (n < 0) panic("recvfrom");
    buf[n] = '\0';
    const char* src_addr =
        inet_ntop(sender_addr.sin_family, &sender_addr.sin_addr,
                  sender_addr_buf, sizeof(sender_addr_buf));
    logline("received %s from %s:%d\n", buf, src_addr, sender_addr.sin_port);
  }
}

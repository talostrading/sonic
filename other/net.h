#ifndef NET_H
#define NET_H

#include <arpa/inet.h>
#include <errno.h>
#include <net/if.h>
#include <netdb.h>
#include <netinet/in.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <unistd.h>

#define MULTICAST_ADDR "224.0.42.42"
#define MULTICAST_PORT 8080

const char* kUdpPort = "8080";  // port we sendto and recvfrom
const int kBufLen = 128;        // max number of bytes we can recv of send

const char* kPanicStr = "panic: \0";

void panic(const char* format, ...) {
  char buf[1024];  // sad
  memset(buf, 0, 1024);
  strcat(buf, kPanicStr);
  strcat(buf, format);
  strcat(buf, "\n");

  va_list arglist;
  va_start(arglist, format);
  vfprintf(stderr, buf, arglist);
  va_end(arglist);
  exit(EXIT_FAILURE);
}

const char* addr_to_str(char* buf, int buf_len, struct sockaddr* addr) {
  void* raw = NULL;
  if (addr->sa_family == AF_INET) {
    raw = &((struct sockaddr_in*)addr)->sin_addr;
  } else if (addr->sa_family == AF_INET6) {
    raw = &((struct sockaddr_in6*)addr)->sin6_addr;
  } else {
    panic("invalid sa_family=%d", addr->sa_family);
  }
  return inet_ntop(addr->sa_family, raw, buf, buf_len);
}

// sendall wraps sendto to ensure all data is send to the other peer.
int sendall(
    int sockfd,
    const char*
        buf /* assume is char*, otherwise pointer arithmetic won't work */,
    size_t len, int flags, const struct sockaddr* dest_addr,
    socklen_t addr_len) {
  int left = len;
  while (left > 0) {
    int n = sendto(sockfd, buf, left, flags, dest_addr, addr_len);
    printf("sending %d\n", n);
    if (n < 0) return n;
    left -= n;
    buf += n;
  }
  return len;
}

void get_sock_info(int sockfd) {
  {
    printf("--------- getting interface info ---------\n");

    char buf[4096];
    struct ifconf conf;
    conf.ifc_len = sizeof(buf);
    conf.ifc_ifcu.ifcu_buf = (caddr_t)buf;
    int ret = ioctl(sockfd, SIOCGIFCONF, &conf);
    if (ret < 0) panic("ioctl interface info");

    char addr_buf[4096];
    struct ifreq* ifr = conf.ifc_ifcu.ifcu_req;
    int index = 0;
    while ((char*)ifr < buf + conf.ifc_len) {
      // get interface capabilities:
      // https://man7.org/linux/man-pages/man7/netdevice.7.html
      struct ifreq icap;
      strncpy(icap.ifr_name, ifr->ifr_name, sizeof(ifr->ifr_name));
      ret = ioctl(sockfd, SIOCGIFFLAGS, &icap);
      if (ret < 0) panic("ioctl interface capabilities");

      int is_up = (icap.ifr_ifru.ifru_flags & IFF_UP) == IFF_UP;
      int is_loopback =
          (icap.ifr_ifru.ifru_flags & IFF_LOOPBACK) == IFF_LOOPBACK;
      int is_point_to_point =
          (icap.ifr_ifru.ifru_flags & IFF_POINTOPOINT) == IFF_POINTOPOINT;

      // if yes, then the interface passes all traffic to the CPU
      int is_promisc = (icap.ifr_ifru.ifru_flags & IFF_PROMISC) == IFF_PROMISC;

      // not really sure what this does, routing stuff
      int is_all_multicast =
          (icap.ifr_ifru.ifru_flags & IFF_ALLMULTI) == IFF_ALLMULTI;

      int is_multicast =
          (icap.ifr_ifru.ifru_flags & IFF_MULTICAST) == IFF_MULTICAST;

      printf(
          "name=%s index=%d address=%s up=%d loopback=%d point_to_point=%d "
          "promisc=%d "
          "all_multicast=%d multicast=%d\n",
          ifr->ifr_name, index++,
          inet_ntop(ifr->ifr_ifru.ifru_addr.sa_family,
                    &((struct sockaddr_in*)&ifr->ifr_ifru.ifru_addr)->sin_addr,
                    addr_buf, sizeof(addr_buf)),
          is_up, is_loopback, is_point_to_point, is_promisc, is_all_multicast,
          is_multicast);

      ifr = (struct ifreq*)((char*)ifr + _SIZEOF_ADDR_IFREQ(*ifr));
    }

    printf("--------------------------------------\n");
  }

  struct ip_mreqn mreq;
  socklen_t len = sizeof(mreq);
  int ret = getsockopt(sockfd, IPPROTO_IP, IP_MULTICAST_IF, &mreq, &len);
  if (ret < 0) {
    if (errno == EBADF) {
    } else if (errno == EFAULT) {
      panic("getsockopt EFAULT");
    } else if (errno == EINVAL) {
      panic("getsockopt EINVAL");
    } else if (errno == ENOPROTOOPT) {
      panic("getsockopt ENOPROTOOPT");
    } else if (errno == ENOTSOCK) {
      panic("getsockopt ENOTSOCK");
    } else {
      panic("getsockopt unknown error");
    }
  };
  char addr_buf[128];
  const char* addr_str = inet_ntop(AF_INET, &mreq.imr_address, addr_buf, 128);
  if (addr_str == NULL) panic("inet_ntop");
  printf("interface index=%d addr=%s\n", mreq.imr_ifindex, addr_str);
}

#endif
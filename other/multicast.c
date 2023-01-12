/* Notes:
- required for ipv6, optional for ipv4
- multicast range: 224.0.0.0 through 239.255.255.255 for ipv4. ipv6 is below
- multicast address + UDP port = session

## Address for ipv6
ipv6 is 128 bits, below each |..| is 8 bits so 1 byte. x means ignored
|ff|flags+scope|x|x|x|x|x|x|x|x|x|x|g|g|g|g|
g = groupid

receiver needs to join with setsockopt
the multicast address from the transport layer is mapped to an Ethernet address.
when a receiver joins, it tells the ethernet layer to receive packets destined
to ethernet_mapped(multicast_addr)

receiver and sender need to bind to the same port.

sender now sends stuff to the multicast address and port.

ethernet cards which are not told to accept packets meant for
ethernet_mapped(multicast_addr) simply discard the packets.

the receiving IP layer remaps the ethernet address to the IPvX address and looks
up if any applications need data from this address/port (i.e. if any have
joined)

more code here:
http://www.cs.kent.edu/~javed/internetbook/programs/TCP-client-server/unp.h

 */
#include "net.h"

int multicast_join(int sockfd, struct sockaddr* const group_addr,
                   int interface_index, const char* interface_name) {
  switch (group_addr->sa_family) {
    case AF_INET:  // IPv4
      struct ip_mreq mreq;
      struct ifreq interface;

      memcpy(&mreq.imr_multiaddr,
             &((const struct sockaddr_in*)group_addr)->sin_addr,
             sizeof(struct in_addr));

      if (interface_index > 0) {
        // find the interface by index and set its name
        if (if_indextoname(interface_index, interface.ifr_name) == NULL) {
          errno = ENXIO;
          return -1;
        }
      } else if (interface_name != NULL) {
        strncpy(interface.ifr_name, interface_name, IFNAMSIZ);
      } else {
        mreq.imr_interface.s_addr = htonl(INADDR_ANY);
      }

      if (ioctl(sockfd, SIOCGIFADDR, &interface) < 0) {
        return -1;
      }
      memcpy(&mreq.imr_interface,
             &((struct sockaddr_in*)&interface.ifr_ifru.ifru_addr)->sin_addr,
             sizeof(struct in_addr));

      return setsockopt(sockfd, IPPROTO_IP, IP_ADD_MEMBERSHIP, &mreq,
                        sizeof(mreq));
      break;
    case AF_INET6:  // IPv6
                    // TODO
      break;
    default:
      return -1;
  }
}

int main(void) {
  struct ipv6_mreq mreq;
  memcpy(&mreq.ipv6mr_multiaddr)
}

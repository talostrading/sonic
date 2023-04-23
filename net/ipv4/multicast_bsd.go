package ipv4

// TODO
type IPMreqn struct {
	Multiaddr [4]byte /* in_addr */
	Interface [4]byte /* in_addr */
	Ifindex   int32
}

package multicast

type Stats struct {
	async struct {
		immediateReads int
		scheduledReads int

		immediateWrites int
		scheduledWrites int
	}
}

func (s *Stats) Reset() {
	s.async.immediateReads = 0
	s.async.scheduledReads = 0

	s.async.immediateWrites = 0
	s.async.scheduledWrites = 0
}

// AsyncReadPerf gives an indication of the async read performance of the peer. Higher is better, and indicates
// that less syscalls have been made.
//
// In the limit:
// - 0.0 means that all reads have passed through the reactor, hence none were done immediately.
// - 1.0 means that none of the reads have passed through the reactor and were hence done immediately.
func (s *Stats) AsyncReadPerf() float64 {
	return 1.0 - float64(s.AsyncScheduledReads())/float64(s.AsyncTotalReads())*2.0
}

func (s *Stats) AsyncTotalReads() int {
	return s.async.immediateReads + s.async.scheduledReads
}

func (s *Stats) AsyncImmediateReads() int {
	return s.async.immediateReads
}

func (s *Stats) AsyncScheduledReads() int {
	return s.async.scheduledReads
}

func (s *Stats) AsyncTotalWrites() int {
	return s.async.immediateWrites + s.async.scheduledWrites
}

func (s *Stats) AsyncImmediateWrites() int {
	return s.async.immediateWrites
}

func (s *Stats) AsyncScheduledWrites() int {
	return s.async.scheduledWrites
}

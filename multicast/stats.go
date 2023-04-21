package multicast

type Stats struct {
	AsyncImmediateReads int
	AsyncScheduledReads int

	AsyncImmediateWrites int
	AsyncScheduledWrites int
}

func (s *Stats) Reset() {
	s.AsyncImmediateReads = 0
	s.AsyncScheduledReads = 0

	s.AsyncImmediateWrites = 0
	s.AsyncScheduledWrites = 0
}

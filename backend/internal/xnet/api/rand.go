package api

// randInt64 returns a pseudo-random int64 with a small lock-protected RNG.
func (s *Server) randInt64() int64 {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Int63()
}

// randintInt returns a pseudo-random int in [0, max).
func (s *Server) randintInt(max int) int {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Intn(max)
}

// randFloat returns a pseudo-random float in [min, max).
func (s *Server) randFloat(min, max float64) float64 {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return min + s.rng.Float64()*(max-min)
}

package api

// randInt64 使用一个小型锁保护的 RNG 返回一个伪随机 int64。
func (s *Server) randInt64() int64 {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Int63()
}

// randintInt 返回 [0, max) 范围内的伪随机整数。
func (s *Server) randintInt(max int) int {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Intn(max)
}

// randFloat 返回 [min, max) 范围内的伪随机浮点数。
func (s *Server) randFloat(min, max float64) float64 {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return min + s.rng.Float64()*(max-min)
}

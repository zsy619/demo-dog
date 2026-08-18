package api

import "github.com/zsy619/demo-dog/backend/internal/stream"

// InjectSeed generates a small batch of seed data and writes it synchronously.
// Useful for the boot-time --seed flag so the UI has data on first load.
func (s *Server) InjectSeed(service string, n int) {
	req := s.generateSeed(service, n)
	s.ingest.SubmitSync(req)
	for _, m := range req.Metrics {
		s.hub.Publish(stream.Event{Kind: "metric", Service: m.Service, Timestamp: m.Timestamp.UnixMilli(), Name: m.Name, Value: m.Value})
	}
	for _, l := range req.Logs {
		s.hub.Publish(stream.Event{Kind: "log", Service: l.Service, Timestamp: l.Timestamp.UnixMilli(), Body: l.Body, Status: string(l.Severity)})
	}
}

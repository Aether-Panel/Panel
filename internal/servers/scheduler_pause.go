package servers

// SetPaused pauses or resumes the server's tasks and saves that with them. A node transfer
// pauses them on both nodes while it copies the server, and resumes them where the server
// ends up.
func (s *Scheduler) SetPaused(paused bool) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.Paused == paused {
		return nil
	}
	s.Paused = paused
	return s.save()
}

// IsPaused reports whether the server's tasks are paused
func (s *Scheduler) IsPaused() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.Paused
}

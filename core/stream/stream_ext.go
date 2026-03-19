package stream

import "os"

// UnderlyingFile returns the underlying *os.File if the stream wraps one, nil otherwise.
func (s *Stream) UnderlyingFile() *os.File {
	if f, ok := s.f.(*os.File); ok {
		return f
	}
	return nil
}

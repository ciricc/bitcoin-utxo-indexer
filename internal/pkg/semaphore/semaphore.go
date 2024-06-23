package semaphore

type Semaphore struct {
	c chan struct{}
}

func New(n int64) *Semaphore {
	return &Semaphore{
		c: make(chan struct{}, n),
	}
}

func (s *Semaphore) Acquire() {
	s.c <- struct{}{}
}

func (s *Semaphore) Release() {
	<-s.c
}

func (s *Semaphore) Close() {
	close(s.c)
}

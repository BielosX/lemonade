package internal

import "sync"

type OneShot struct {
	done chan struct{}
	once sync.Once
}

func NewOneShot() *OneShot {
	return &OneShot{
		done: make(chan struct{}),
	}
}

func (o *OneShot) Done() <-chan struct{} {
	return o.done
}

func (o *OneShot) Signal() {
	o.once.Do(func() {
		close(o.done)
	})
}

package stack

import (
	"iter"
	"sync"
	"time"
)

type Stack[T any] interface {
	Push(v T)
	Pop() (T, bool)
	Len() int
}

type SliceStack[T any] []T

func New[T any]() Stack[T] {
	return NewSlice[T]()
}

func NewSlice[T any]() *SliceStack[T] {
	return &SliceStack[T]{}
}
func (s *SliceStack[T]) Push(v T) {
	// s.data = append(s.data, v)
	*s = append(*s, v)
}

func (s *SliceStack[T]) Pop() (T, bool) {
	if len(*s) == 0 {
		var zero T
		return zero, false
	}
	v := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]

	return v, true
}
func (s *SliceStack[T]) Len() int {
	return len(*s)
}

type SyncStack[T any] struct {
	stack *SliceStack[T]
	mtx   sync.Mutex
}

func NewSync[T any]() *SyncStack[T] {
	return &SyncStack[T]{
		stack: NewSlice[T](),
	}
}

func (s *SyncStack[T]) Push(v T) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.stack.Push(v)
}

func (s *SyncStack[T]) Pop() (T, bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.stack.Pop()
}
func (s *SyncStack[T]) Len() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.stack.Len()
}

type SyncDoneStack[T any] struct {
	*SyncStack[T]
	done bool
}

func NewSyncDone[T any]() *SyncDoneStack[T] {
	return &SyncDoneStack[T]{
		SyncStack: NewSync[T](),
	}
}

func (s *SyncDoneStack[T]) Done() bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.done
}

func (s *SyncDoneStack[T]) Finish(done bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.done = done
}

func (s *SyncDoneStack[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		var v T
		var ok bool
		for {
			v, ok = s.Pop()
			if !ok && s.done {
				return
			}
			if !ok {
				time.Sleep(time.Millisecond * 100)
				continue
			}
			yield(v)
		}
	}
}

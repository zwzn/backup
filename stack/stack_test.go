package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStack(t *testing.T) {
	s := New[int]()
	s.Push(7)

	assert.Equal(t, 1, s.Len())

	s.Push(14)

	assert.Equal(t, 2, s.Len())

	v, ok := s.Pop()
	assert.Equal(t, 14, v)
	assert.Equal(t, true, ok)

	assert.Equal(t, 1, s.Len())

	v, ok = s.Pop()
	assert.Equal(t, 7, v)
	assert.Equal(t, true, ok)

	assert.Equal(t, 0, s.Len())

	v, ok = s.Pop()
	assert.Equal(t, 0, v)
	assert.Equal(t, false, ok)

	assert.Equal(t, 0, s.Len())

	s.Push(9)

	assert.Equal(t, 1, s.Len())

	v, ok = s.Pop()
	assert.Equal(t, 9, v)
	assert.Equal(t, true, ok)
}

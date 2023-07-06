// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import "errors"

type Queue[T any] interface {
	// Size returns the amount of elements currently in the queue.
	Size() int
	// Peek returns the first element without removing it.
	// Returns an error if the queue is empty.
	Peek() (T, error)
	// Poll returns the first element and removes it from the queue.
	// Returns an error if the queue is empty.
	Poll() (T, error)
	// Push adds the given elements to the queue in the given order.
	Push(...T)
	// Clear removes all elements from the queue.
	Clear()
}

var ErrQueueEmpty = errors.New("queue is empty")

var _ Queue[any] = &BasicQueue[any]{}

type element[T any] struct {
	next  *element[T]
	value T
}

type BasicQueue[T any] struct {
	size        int
	first, last *element[T]
}

func NewQueue[T any](elems ...T) Queue[T] {
	head, tail := toElements[T](elems)
	return &BasicQueue[T]{
		size:  len(elems),
		first: head,
		last:  tail,
	}
}

// Size returns the amount of elements currently in the queue.
func (q *BasicQueue[T]) Size() int {
	return q.size
}

// Peek returns the first element without removing it.
// Returns an error if the queue is empty.
func (q *BasicQueue[T]) Peek() (T, error) {
	if q.first == nil {
		var zero T
		return zero, ErrQueueEmpty
	}
	return q.first.value, nil
}

// Poll returns the first element and removes it from the queue.
// Returns an error if the queue is empty.
func (q *BasicQueue[T]) Poll() (T, error) {
	if q.first == nil {
		var zero T
		return zero, ErrQueueEmpty
	}
	res := q.first.value
	q.first = q.first.next
	q.size--
	return res, nil
}

// Push adds the given elements to the queue in the given order.
func (q *BasicQueue[T]) Push(elems ...T) {
	if len(elems) == 0 {
		return
	}
	head, tail := toElements[T](elems)
	if q.Size() == 0 {
		q.first = head
	} else {
		q.last.next = head
	}
	q.last = tail
	q.size += len(elems)
}

// Clear removes all elements from the queue.
func (q *BasicQueue[T]) Clear() {
	q.first = nil
	q.last = nil
	q.size = 0
}

// toElements converts a slice of T to a linked list style chain of elements[T].
// If the given list is empty, nil is returned.
// Otherwise, it returns a pointer to the first and last element of the chain.
func toElements[T any](elems []T) (*element[T], *element[T]) {
	if len(elems) == 0 {
		return nil, nil
	}
	first := &element[T]{
		value: elems[0],
	}
	last := first
	for _, elem := range elems[1:] {
		new := &element[T]{
			value: elem,
		}
		last.next = new
		last = new
	}
	return first, last
}

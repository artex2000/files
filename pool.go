package main

import (
	"sync"
)

type worker struct {
	parent *pool
	input  chan string
}

type pool struct {
	wg      *sync.WaitGroup
	workers []*worker
	queue   chan *worker
}

func newPool(count int) *pool {
	r := &pool{
		wg:    &sync.WaitGroup{},
		queue: make(chan *worker, count),
	}

	for i := 0; i < count; i++ {
		r.wg.Add(1)
		w := &worker{
			parent: r,
			input:  make(chan string),
		}
		r.workers = append(r.workers, w)
	}
	return r
}

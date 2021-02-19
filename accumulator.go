package main

import (
	"math"
)

type accumulatorCounter struct {
	summ    uint64
	updates uint64
	max     uint64
	min     uint64
}

// This accumulator is fast, but not thread safe. Race when
// calling Tick() and Add() and between calls to Add() produces not reliable result
type Accumulator struct {
	counters []accumulatorCounter
	cursor   uint64
	size     uint64
	count    uint64
}

type Result struct {
	nonzero   bool
	maxWindow uint64
	max       uint64
	min       uint64
	results   []uint64
}

const DaysInWeek = 7

func NewAccumulator() *Accumulator {
	a := &Accumulator{
		counters: make([]accumulatorCounter, DaysInWeek),
		size:     DaysInWeek,
		count:    0,
	}
	a.Reset()
	return a
}

func (a *Accumulator) Reset() {
	a.cursor = 0
	a.count = 0
	// Probably faster for lager arrays than call to make()
	for i := uint64(0); i < a.size; i++ {
		counter := &a.counters[i]
		counter.summ = 0
		counter.updates = 0
		counter.max = uint64(0)
		counter.min = uint64(math.MaxUint64)
	}
}

func (a *Accumulator) incCursor(cursor uint64) uint64 {
	if cursor >= (a.size - 1) {
		return 0
	} else {
		return (cursor + 1)
	}
}

func (a *Accumulator) decCursor(cursor uint64) uint64 {
	if cursor > (0) {
		return cursor - 1
	} else {
		return (a.size - 1)
	}
}

func (a *Accumulator) getResult(average bool) Result {
	var nonzero = false
	size := a.size
	var cursor = a.cursor
	if size > a.count {
		size = a.count
		cursor = a.size
	}
	results := make([]uint64, size)
	max := uint64(0)
	var min uint64 = math.MaxUint64
	maxWindow := uint64(0)
	for i := uint64(0); i < size; i++ {
		cursor = a.incCursor(cursor)
		updates := a.counters[cursor].updates
		if updates > 0 {
			nonzero = true
			summ := a.counters[cursor].summ
			if maxWindow < summ {
				maxWindow = summ
			}
			var result uint64
			if average {
				result = (summ / updates)
			} else {
				result = (summ)
			}
			if max < result {
				max = result
			}
			if min > result {
				min = result
			}
			results[i] = result
		} else {
			results[i] = 0
		}
	}
	return Result{
		results:   results,
		nonzero:   nonzero,
		max:       max,
		min:       min,
		maxWindow: maxWindow,
	}
}

func (a *Accumulator) Add(value uint64) {
	cursor := a.cursor
	counter := &a.counters[cursor]
	counter.summ += value
	counter.updates++
	if value > counter.max {
		counter.max = value
	}
	if value < counter.min {
		counter.min = value
	}
}

func (a *Accumulator) Tick() {
	cursor := a.incCursor(a.cursor)
	a.cursor = cursor
	a.counters[cursor].summ = 0
	a.counters[cursor].updates = 0
	a.count++
}

package accumulator

import (
	"bytes"
	"fmt"
	"math"
	"sync"
)

type accumulatorCounter struct {
	summ    uint64
	updates uint64
}

// This accumulator is fast, but not thread safe. Race when
// calling Tick() and Add() and between calls to Add() produces not reliable result
// Use InitSync(), TickSync() and AddSync() if thread safety is desired
type Accumulator struct {
	counters []accumulatorCounter
	cursor   uint64
	size     uint64
	count    uint64
	mutex    *sync.Mutex
	Name     string
}

type Result struct {
	Nonzero   bool
	MaxWindow uint64
	Max       uint64
	Min       uint64
	Results   []uint64
}

func New(name string, size uint64) *Accumulator {
	a := &Accumulator{
		counters: make([]accumulatorCounter, size),
		size:     size,
		count:    0,
		Name:     name,
	}
	a.Reset()
	return a
}

func NewSync(name string, size uint64) *Accumulator {
	a := New(name, size)
	a.mutex = &sync.Mutex{}
	return a
}

func (a *Accumulator) Reset() {
	a.cursor = 0
	a.count = 0
	// Probably faster than call to make()
	for i := uint64(0); i < a.size; i++ {
		a.counters[i].summ = 0
		a.counters[i].updates = 0
	}
}

func (a *Accumulator) Size() uint64 {
	return a.size
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

// Return the accumulator for the last Tick
func (a *Accumulator) PeekSumm() uint64 {
	cursor := a.decCursor(a.cursor)
	return a.counters[cursor].summ
}

// Return average for the last Tick
func (a *Accumulator) PeekAverage() uint64 {
	cursor := a.decCursor(a.cursor)
	return (a.counters[cursor].summ / a.counters[cursor].updates)
}

// Return the results - averages over the window of "size" entries
// Use "divider" to normalize the output in the same copy path
func (a *Accumulator) GetAverage(divider uint64) Result {
	return a.getResult(divider, true)
}

// Use "divider" to normalize the output in the same copy path
func (a *Accumulator) GetSumm(divider uint64) Result {
	return a.getResult(divider, false)
}

func (a *Accumulator) GetSummSync(divider uint64) Result {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.getResult(divider, false)
}

// Use "divider" to normalize the output in the same copy path
func (a *Accumulator) getResult(divider uint64, average bool) Result {
	var nonzero = false
	if divider == 0 {
		divider = 1
	}
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
				result = (summ / (divider * updates))
			} else {
				result = (summ / divider)
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
		Results:   results,
		Nonzero:   nonzero,
		Max:       max,
		Min:       min,
		MaxWindow: maxWindow,
	}
}

func (a *Accumulator) Add(value uint64) {
	cursor := a.cursor
	a.counters[cursor].summ += value
	a.counters[cursor].updates++
}

func (a *Accumulator) AddSync(value uint64) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.Add(value)
}

func (a *Accumulator) Tick() {
	cursor := a.incCursor(a.cursor)
	a.cursor = cursor
	a.counters[cursor].summ = 0
	a.counters[cursor].updates = 0
	a.count++
}

func (a *Accumulator) TickSync() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.Tick()
}

func SprintfSliceUint64(valueFormat string, columns int, a []uint64) string {
	if valueFormat == "" {
		valueFormat = "%d "
	}
	if columns <= 0 {
		columns = 4
	}
	var buffer bytes.Buffer
	for col, v := range a {
		if (col%columns == 0) && (col != 0) {
			buffer.WriteString("\n")
		}
		buffer.WriteString(fmt.Sprintf(valueFormat, v))
	}
	s := buffer.String()
	s = s[:(len(s) - 1)]
	return s
}

func (a *Accumulator) Sprintf(nameFormat string, noDataFormat string, valueFormat string, columns int, divider uint64, average bool) string {
	var result Result
	if average {
		result = a.GetAverage(divider)
	} else {
		result = a.GetSumm(divider)
	}
	if nameFormat == "" {
		nameFormat = "%s\n\t%v                         \n"
	}
	if noDataFormat == "" {
		noDataFormat = "%s\n\tNo requests in the last %d seconds\n"
	}

	if result.Nonzero {
		return fmt.Sprintf(nameFormat, a.Name, SprintfSliceUint64(valueFormat, columns, result.Results))
	} else {
		return fmt.Sprintf(noDataFormat, a.Name, a.Size())
	}
}

// This is a replacement for the Prometheus "histogram" metric
func (a *Accumulator) Prometheus(name string, comment string) string {
	results := a.GetAverage(1)
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("# HELP %s %s\n", name, comment))
	buffer.WriteString(fmt.Sprintf("# TYPE %s histogram\n", name))
	for bin, result := range results.Results {
		value := result
		upperLimit := bin
		buffer.WriteString(fmt.Sprintf("%s_bucket{le=\"%d\"} %d\n", name, upperLimit, value))
	}
	// TODO Collect the update count and sums
	buffer.WriteString(fmt.Sprintf("%s_sum %d\n", name, -1))
	buffer.WriteString(fmt.Sprintf("%s_count %d\n", name, -1))
	return buffer.String()
}

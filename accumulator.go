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

// I keep a dedicated accumulator for every sensor or ~250 bytes/sensor
// This accumulator is fast, but not thread safe. Race when
// calling Tick() and Add() and in between calls to Add() produces not reliable result
type Accumulator struct {
	// Shortcut: I need statically allocated arrays/memory pool
	// Shoortcut: it is not data cache efficient. I want counters ordered by day of week and not by sensor
	counters []accumulatorCounter
	cursor   uint64
	size     uint64
	count    uint64
}

type Result struct {
	nonzero bool

	windowMax     uint64
	windowMin     uint64
	windowAverage uint64

	max     []uint64
	min     []uint64
	average []uint64
}

func NewAccumulator(size uint64) *Accumulator {
	a := &Accumulator{
		counters: make([]accumulatorCounter, size),
		size:     size,
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

func (a *Accumulator) getResult() Result {
	var nonzero = false
	size := a.size
	var cursor = a.cursor
	if size > a.count {
		size = a.count
		cursor = a.size
	}
	average := make([]uint64, size)
	min := make([]uint64, size)
	max := make([]uint64, size)
	windowMax := uint64(0)
	var windowMin uint64 = math.MaxUint64
	windowSumm := uint64(0)
	for i := uint64(0); i < size; i++ {
		cursor = a.incCursor(cursor)
		updates := a.counters[cursor].updates
		if updates > 0 {
			nonzero = true
			counter := &a.counters[cursor]
			summ := counter.summ
			if windowMax < counter.max {
				windowMax = counter.max
			}
			if windowMin > counter.min {
				windowMin = counter.min
			}
			counterAverage := (summ / updates)
			average[i] = counterAverage
			min[i] = counter.min
			max[i] = counter.max
			windowSumm += counterAverage
		} else {
			average[i] = 0
			max[i] = 0
			min[i] = 0
		}
	}
	windowAverage := uint64(0)
	if size > 0 {
		windowAverage = windowSumm / size
	}
	return Result{
		nonzero:       nonzero,
		max:           max,
		min:           min,
		windowMin:     windowMin,
		windowMax:     windowMax,
		windowAverage: windowAverage,
		average:       average,
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

func celsius2MilliKelvin(c float64) uint64 {
	return uint64((1000 * (c + 273.15)))
}

func milliKelvin2Celsius(mk uint64) float64 {
	return ((float64(mk) - 1000*273.15) / 1000)
}

func milliKelvin2CelsiusSlice(data []uint64) []float64 {
	result := make([]float64, len(data))
	for idx, mk := range data {
		result[idx] = milliKelvin2Celsius(mk)
	}

	return result
}

type ResultCelcius struct {
	nonzero bool

	windowMax     float64
	windowMin     float64
	windowAverage float64

	max     []float64
	min     []float64
	average []float64
}

func milliKelvin2CelsiusResult(result Result) ResultCelcius {
	resultCelcius := ResultCelcius{
		nonzero:       result.nonzero,
		windowMax:     milliKelvin2Celsius(result.windowMax),
		windowMin:     milliKelvin2Celsius(result.windowMin),
		windowAverage: milliKelvin2Celsius(result.windowAverage),
		max:           milliKelvin2CelsiusSlice(result.max),
		min:           milliKelvin2CelsiusSlice(result.min),
		average:       milliKelvin2CelsiusSlice(result.average),
	}
	return resultCelcius
}

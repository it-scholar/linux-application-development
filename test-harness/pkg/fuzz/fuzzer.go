// Package fuzz provides fuzz testing capabilities
package fuzz

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Fuzzer generates fuzz test inputs
type Fuzzer struct {
	seed       int64
	rng        *rand.Rand
	strategies []FuzzStrategy
}

// FuzzStrategy represents a fuzzing strategy
type FuzzStrategy int

const (
	StrategyRandom FuzzStrategy = iota
	StrategyBoundary
	StrategyBitFlip
	StrategyByteFlip
	StrategyArithmetic
	StrategyDictionary
)

// Input represents a fuzz test input
type Input struct {
	Data     []byte
	Strategy FuzzStrategy
	Seed     int64
	Metadata map[string]interface{}
}

// Result represents a fuzz test result
type Result struct {
	Input     Input
	Crashed   bool
	Error     error
	Duration  time.Duration
	Timestamp time.Time
}

// Config holds fuzzer configuration
type Config struct {
	Iterations   int
	Duration     time.Duration
	MaxInputSize int
	Strategies   []FuzzStrategy
	Seed         int64
}

// NewFuzzer creates a new fuzzer
func NewFuzzer(config Config) *Fuzzer {
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	strategies := config.Strategies
	if len(strategies) == 0 {
		strategies = []FuzzStrategy{
			StrategyRandom,
			StrategyBoundary,
			StrategyBitFlip,
			StrategyByteFlip,
		}
	}

	return &Fuzzer{
		seed:       seed,
		rng:        rand.New(rand.NewSource(seed)),
		strategies: strategies,
	}
}

// GenerateInput generates a fuzz input using a specific strategy
func (f *Fuzzer) GenerateInput(strategy FuzzStrategy, minSize, maxSize int) *Input {
	size := minSize + f.rng.Intn(maxSize-minSize+1)
	data := make([]byte, size)

	switch strategy {
	case StrategyRandom:
		f.rng.Read(data)
	case StrategyBoundary:
		data = f.generateBoundaryData(size)
	case StrategyBitFlip:
		data = f.generateBitFlipData(size)
	case StrategyByteFlip:
		data = f.generateByteFlipData(size)
	case StrategyArithmetic:
		data = f.generateArithmeticData(size)
	case StrategyDictionary:
		data = f.generateDictionaryData(size)
	default:
		f.rng.Read(data)
	}

	return &Input{
		Data:     data,
		Strategy: strategy,
		Seed:     f.seed,
		Metadata: map[string]interface{}{
			"size": size,
		},
	}
}

func (f *Fuzzer) generateBoundaryData(size int) []byte {
	data := make([]byte, size)

	// Fill with boundary values
	boundaryValues := []byte{0x00, 0x01, 0x7F, 0x80, 0xFE, 0xFF}
	for i := 0; i < size; i++ {
		data[i] = boundaryValues[f.rng.Intn(len(boundaryValues))]
	}

	return data
}

func (f *Fuzzer) generateBitFlipData(size int) []byte {
	// Start with valid-ish data and flip bits
	data := make([]byte, size)
	f.rng.Read(data)

	// Flip random bits
	numFlips := 1 + f.rng.Intn(size/2+1)
	for i := 0; i < numFlips; i++ {
		pos := f.rng.Intn(size)
		bit := uint(f.rng.Intn(8))
		data[pos] ^= (1 << bit)
	}

	return data
}

func (f *Fuzzer) generateByteFlipData(size int) []byte {
	data := make([]byte, size)
	f.rng.Read(data)

	// Flip random bytes
	numFlips := 1 + f.rng.Intn(size/4+1)
	for i := 0; i < numFlips; i++ {
		pos := f.rng.Intn(size)
		data[pos] = ^data[pos]
	}

	return data
}

func (f *Fuzzer) generateArithmeticData(size int) []byte {
	data := make([]byte, size)
	f.rng.Read(data)

	// Apply arithmetic mutations
	numMutations := 1 + f.rng.Intn(size/4+1)
	for i := 0; i < numMutations; i++ {
		pos := f.rng.Intn(size)
		op := f.rng.Intn(4)
		switch op {
		case 0:
			data[pos]++
		case 1:
			data[pos]--
		case 2:
			data[pos] += byte(f.rng.Intn(32))
		case 3:
			data[pos] -= byte(f.rng.Intn(32))
		}
	}

	return data
}

func (f *Fuzzer) generateDictionaryData(size int) []byte {
	// Use dictionary of interesting values
	dictionary := [][]byte{
		[]byte("WEAT"),
		[]byte{0x00, 0x00, 0x00, 0x01}, // Version 1
		[]byte{0xFF, 0xFF, 0xFF, 0xFF}, // Max uint32
		[]byte{0x7F, 0xFF, 0xFF, 0xFF}, // Max int32
		[]byte{0x80, 0x00, 0x00, 0x00}, // Min int32
	}

	data := make([]byte, size)
	f.rng.Read(data)

	// Insert dictionary entries
	if size > 4 {
		numInserts := 1 + f.rng.Intn(3)
		for i := 0; i < numInserts && i < len(dictionary); i++ {
			entry := dictionary[f.rng.Intn(len(dictionary))]
			pos := f.rng.Intn(size - len(entry) + 1)
			copy(data[pos:], entry)
		}
	}

	return data
}

// Run executes fuzz tests against a target function
func (f *Fuzzer) Run(ctx context.Context, target func([]byte) error, config Config) ([]Result, error) {
	results := make([]Result, 0)

	iterations := config.Iterations
	if iterations == 0 {
		iterations = 1000
	}

	maxSize := config.MaxInputSize
	if maxSize == 0 {
		maxSize = 1024
	}

	startTime := time.Now()

	for i := 0; i < iterations; i++ {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		// Check duration limit
		if config.Duration > 0 && time.Since(startTime) > config.Duration {
			break
		}

		// Select strategy
		strategy := f.strategies[f.rng.Intn(len(f.strategies))]

		// Generate input
		input := f.GenerateInput(strategy, 1, maxSize)

		// Run target
		result := f.runTarget(target, input)
		results = append(results, result)

		// Stop on crash if configured
		if result.Crashed {
			break
		}
	}

	return results, nil
}

func (f *Fuzzer) runTarget(target func([]byte) error, input *Input) Result {
	start := time.Now()

	// Run with panic recovery
	var err error
	var crashed bool

	func() {
		defer func() {
			if r := recover(); r != nil {
				crashed = true
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		err = target(input.Data)
	}()

	return Result{
		Input:     *input,
		Crashed:   crashed,
		Error:     err,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}
}

// GenerateReport generates a fuzz testing report
func (f *Fuzzer) GenerateReport(results []Result) *Report {
	report := &Report{
		TotalRuns:     len(results),
		Crashes:       0,
		Errors:        0,
		Successes:     0,
		Strategies:    make(map[FuzzStrategy]int),
		CrashedInputs: make([]Input, 0),
	}

	var totalDuration time.Duration

	for _, result := range results {
		totalDuration += result.Duration
		report.Strategies[result.Input.Strategy]++

		if result.Crashed {
			report.Crashes++
			report.CrashedInputs = append(report.CrashedInputs, result.Input)
		} else if result.Error != nil {
			report.Errors++
		} else {
			report.Successes++
		}
	}

	if len(results) > 0 {
		report.AverageDuration = totalDuration / time.Duration(len(results))
	}

	return report
}

// Report represents a fuzz testing report
type Report struct {
	TotalRuns       int
	Crashes         int
	Errors          int
	Successes       int
	AverageDuration time.Duration
	Strategies      map[FuzzStrategy]int
	CrashedInputs   []Input
}

// IsSuccessful returns true if no crashes occurred
func (r *Report) IsSuccessful() bool {
	return r.Crashes == 0
}

// GetCrashRate returns the crash rate as a percentage
func (r *Report) GetCrashRate() float64 {
	if r.TotalRuns == 0 {
		return 0
	}
	return float64(r.Crashes) / float64(r.TotalRuns) * 100
}

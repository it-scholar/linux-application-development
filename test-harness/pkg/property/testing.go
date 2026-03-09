// Package property provides property-based testing capabilities
package property

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Test represents a property-based test
type Test struct {
	name       string
	property   func(interface{}) bool
	generator  Generator
	shrinks    int
	maxShrinks int
}

// Generator generates test values
type Generator interface {
	Generate(rng *rand.Rand) interface{}
	Shrink(value interface{}) []interface{}
}

// Result represents a property test result
type Result struct {
	Passed      bool
	Name        string
	Runs        int
	Shrinks     int
	FailedValue interface{}
	Error       string
	Duration    time.Duration
}

// Config holds property test configuration
type Config struct {
	Iterations int
	MaxShrink  int
	Seed       int64
}

// New creates a new property test
func New(name string, property func(interface{}) bool, generator Generator) *Test {
	return &Test{
		name:       name,
		property:   property,
		generator:  generator,
		maxShrinks: 100,
	}
}

// Run executes the property test
func (t *Test) Run(ctx context.Context, config Config) *Result {
	start := time.Now()

	iterations := config.Iterations
	if iterations == 0 {
		iterations = 100
	}

	maxShrink := config.MaxShrink
	if maxShrink == 0 {
		maxShrink = t.maxShrinks
	}

	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	rng := rand.New(rand.NewSource(seed))

	for i := 0; i < iterations; i++ {
		select {
		case <-ctx.Done():
			return &Result{
				Passed:   false,
				Name:     t.name,
				Runs:     i,
				Error:    "context cancelled",
				Duration: time.Since(start),
			}
		default:
		}

		// Generate test value
		value := t.generator.Generate(rng)

		// Check property
		if !t.property(value) {
			// Property failed, try to shrink
			shrunkValue, shrinks := t.shrink(value, maxShrink)

			return &Result{
				Passed:      false,
				Name:        t.name,
				Runs:        i + 1,
				Shrinks:     shrinks,
				FailedValue: shrunkValue,
				Error:       fmt.Sprintf("property failed for value: %v", shrunkValue),
				Duration:    time.Since(start),
			}
		}
	}

	return &Result{
		Passed:   true,
		Name:     t.name,
		Runs:     iterations,
		Duration: time.Since(start),
	}
}

func (t *Test) shrink(value interface{}, maxShrinks int) (interface{}, int) {
	shrinks := 0
	current := value

	for shrinks < maxShrinks {
		candidates := t.generator.Shrink(current)
		if len(candidates) == 0 {
			break
		}

		foundSmaller := false
		for _, candidate := range candidates {
			if !t.property(candidate) {
				current = candidate
				shrinks++
				foundSmaller = true
				break
			}
		}

		if !foundSmaller {
			break
		}
	}

	return current, shrinks
}

// IntGenerator generates random integers
type IntGenerator struct {
	Min int
	Max int
}

func (g *IntGenerator) Generate(rng *rand.Rand) interface{} {
	return rng.Intn(g.Max-g.Min+1) + g.Min
}

func (g *IntGenerator) Shrink(value interface{}) []interface{} {
	v := value.(int)
	var candidates []interface{}

	if v > g.Min {
		candidates = append(candidates, v/2)
		candidates = append(candidates, v-1)
	}
	if v < 0 {
		candidates = append(candidates, -v)
	}

	return candidates
}

// StringGenerator generates random strings
type StringGenerator struct {
	MinLen int
	MaxLen int
	Chars  string
}

func (g *StringGenerator) Generate(rng *rand.Rand) interface{} {
	if g.Chars == "" {
		g.Chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}

	length := g.MinLen + rng.Intn(g.MaxLen-g.MinLen+1)
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = g.Chars[rng.Intn(len(g.Chars))]
	}
	return string(result)
}

func (g *StringGenerator) Shrink(value interface{}) []interface{} {
	s := value.(string)
	var candidates []interface{}

	if len(s) > g.MinLen {
		// Remove last character
		candidates = append(candidates, s[:len(s)-1])
		// Remove first character
		candidates = append(candidates, s[1:])
		// Take half
		candidates = append(candidates, s[:len(s)/2])
	}

	return candidates
}

// SliceGenerator generates random slices
type SliceGenerator struct {
	MinLen     int
	MaxLen     int
	ElementGen Generator
}

func (g *SliceGenerator) Generate(rng *rand.Rand) interface{} {
	length := g.MinLen + rng.Intn(g.MaxLen-g.MinLen+1)
	result := make([]interface{}, length)
	for i := 0; i < length; i++ {
		result[i] = g.ElementGen.Generate(rng)
	}
	return result
}

func (g *SliceGenerator) Shrink(value interface{}) []interface{} {
	slice := value.([]interface{})
	var candidates []interface{}

	if len(slice) > g.MinLen {
		// Remove last element
		candidates = append(candidates, slice[:len(slice)-1])
		// Remove first element
		candidates = append(candidates, slice[1:])
		// Take half
		candidates = append(candidates, slice[:len(slice)/2])

		// Try shrinking individual elements
		for i, elem := range slice {
			shrunkElems := g.ElementGen.Shrink(elem)
			for _, shrunk := range shrunkElems {
				newSlice := make([]interface{}, len(slice))
				copy(newSlice, slice)
				newSlice[i] = shrunk
				candidates = append(candidates, newSlice)
			}
		}
	}

	return candidates
}

// MapGenerator generates random maps
type MapGenerator struct {
	MinLen   int
	MaxLen   int
	KeyGen   Generator
	ValueGen Generator
}

func (g *MapGenerator) Generate(rng *rand.Rand) interface{} {
	length := g.MinLen + rng.Intn(g.MaxLen-g.MinLen+1)
	result := make(map[interface{}]interface{})
	for i := 0; i < length; i++ {
		key := g.KeyGen.Generate(rng)
		value := g.ValueGen.Generate(rng)
		result[key] = value
	}
	return result
}

func (g *MapGenerator) Shrink(value interface{}) []interface{} {
	m := value.(map[interface{}]interface{})
	var candidates []interface{}

	if len(m) > g.MinLen {
		// Remove random entries
		for k := range m {
			newMap := make(map[interface{}]interface{})
			for k2, v2 := range m {
				if k2 != k {
					newMap[k2] = v2
				}
			}
			candidates = append(candidates, newMap)
			break
		}
	}

	return candidates
}

// OneOfGenerator chooses randomly from multiple generators
type OneOfGenerator struct {
	Generators []Generator
}

func (g *OneOfGenerator) Generate(rng *rand.Rand) interface{} {
	if len(g.Generators) == 0 {
		return nil
	}
	return g.Generators[rng.Intn(len(g.Generators))].Generate(rng)
}

func (g *OneOfGenerator) Shrink(value interface{}) []interface{} {
	// Try to find which generator produced this value
	for _, gen := range g.Generators {
		candidates := gen.Shrink(value)
		if len(candidates) > 0 {
			return candidates
		}
	}
	return nil
}

// Runner runs multiple property tests
type Runner struct {
	tests []*Test
}

// NewRunner creates a new property test runner
func NewRunner() *Runner {
	return &Runner{
		tests: make([]*Test, 0),
	}
}

// Add adds a test to the runner
func (r *Runner) Add(test *Test) {
	r.tests = append(r.tests, test)
}

// Run executes all property tests
func (r *Runner) Run(ctx context.Context, config Config) []*Result {
	results := make([]*Result, 0, len(r.tests))

	for _, test := range r.tests {
		result := test.Run(ctx, config)
		results = append(results, result)
	}

	return results
}

// GenerateReport generates a summary report
func GenerateReport(results []*Result) *SummaryReport {
	report := &SummaryReport{
		TotalTests: len(results),
		Passed:     0,
		Failed:     0,
		Results:    results,
	}

	for _, result := range results {
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}

	return report
}

// SummaryReport represents a summary of property tests
type SummaryReport struct {
	TotalTests int
	Passed     int
	Failed     int
	Results    []*Result
}

// AllPassed returns true if all tests passed
func (s *SummaryReport) AllPassed() bool {
	return s.Failed == 0
}

// ForAll is a helper for creating property tests
func ForAll(name string, generator Generator, property func(interface{}) bool) *Test {
	return New(name, property, generator)
}

// ForAll2 tests a property over two generators
func ForAll2(name string, gen1, gen2 Generator, property func(interface{}, interface{}) bool) *Test {
	combined := &combinedGenerator{gen1: gen1, gen2: gen2}
	return New(name, func(v interface{}) bool {
		pair := v.(pair)
		return property(pair.first, pair.second)
	}, combined)
}

type pair struct {
	first  interface{}
	second interface{}
}

type combinedGenerator struct {
	gen1 Generator
	gen2 Generator
}

func (g *combinedGenerator) Generate(rng *rand.Rand) interface{} {
	return pair{
		first:  g.gen1.Generate(rng),
		second: g.gen2.Generate(rng),
	}
}

func (g *combinedGenerator) Shrink(value interface{}) []interface{} {
	p := value.(pair)
	var candidates []interface{}

	// Shrink first
	for _, shrunk := range g.gen1.Shrink(p.first) {
		candidates = append(candidates, pair{first: shrunk, second: p.second})
	}

	// Shrink second
	for _, shrunk := range g.gen2.Shrink(p.second) {
		candidates = append(candidates, pair{first: p.first, second: shrunk})
	}

	return candidates
}

// Helpers for common generators

// Int returns an integer generator
func Int(min, max int) Generator {
	return &IntGenerator{Min: min, Max: max}
}

// String returns a string generator
func String(minLen, maxLen int) Generator {
	return &StringGenerator{MinLen: minLen, MaxLen: maxLen}
}

// Slice returns a slice generator
func Slice(minLen, maxLen int, elemGen Generator) Generator {
	return &SliceGenerator{MinLen: minLen, MaxLen: maxLen, ElementGen: elemGen}
}

// Map returns a map generator
func Map(minLen, maxLen int, keyGen, valueGen Generator) Generator {
	return &MapGenerator{MinLen: minLen, MaxLen: maxLen, KeyGen: keyGen, ValueGen: valueGen}
}

// OneOf returns a generator that picks from multiple generators
func OneOf(generators ...Generator) Generator {
	return &OneOfGenerator{Generators: generators}
}

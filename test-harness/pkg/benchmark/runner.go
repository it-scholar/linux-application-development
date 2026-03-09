// Package benchmark provides performance testing capabilities
package benchmark

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
)

// Runner executes performance benchmarks
type Runner struct {
	logger  *log.Logger
	results map[string]*Result
	mu      sync.RWMutex
}

// Result holds benchmark results
type Result struct {
	Name       string
	Duration   time.Duration
	TotalOps   int64
	TotalBytes int64
	Errors     int64
	Latencies  []time.Duration // All latencies for percentile calculation
	Throughput float64         // ops/sec
	p50        time.Duration
	p95        time.Duration
	p99        time.Duration
	Timestamp  time.Time
}

// Config for benchmark runs
type Config struct {
	Duration    time.Duration
	Concurrency int
	WarmUp      time.Duration
	Target      string
	Operation   string
	PayloadSize int64
}

// ServiceTarget defines what to benchmark
type ServiceTarget struct {
	Name      string
	Endpoint  string
	Port      int
	Operation string // ingest, query, etc.
}

// Metrics collected during benchmark
type Metrics struct {
	ops       int64
	errors    int64
	bytes     int64
	latencies []time.Duration
	mu        sync.Mutex
}

// NewRunner creates a new benchmark runner
func NewRunner(logger *log.Logger) *Runner {
	if logger == nil {
		logger = log.New(os.Stderr)
	}
	return &Runner{
		logger:  logger,
		results: make(map[string]*Result),
	}
}

// RunIngestBenchmark benchmarks CSV ingestion throughput
func (r *Runner) RunIngestBenchmark(ctx context.Context, target ServiceTarget, cfg Config) (*Result, error) {
	r.logger.Info("starting ingest benchmark",
		"target", target.Name,
		"duration", cfg.Duration,
		"concurrency", cfg.Concurrency,
	)

	metrics := &Metrics{
		latencies: make([]time.Duration, 0, 10000),
	}

	// Warm-up phase
	if cfg.WarmUp > 0 {
		r.logger.Info("warm-up phase", "duration", cfg.WarmUp)
		warmCtx, cancel := context.WithTimeout(ctx, cfg.WarmUp)
		r.runWorkers(warmCtx, target, cfg, metrics, r.ingestOperation)
		cancel()
		r.logger.Info("warm-up complete")

		// Reset metrics after warm-up
		atomic.StoreInt64(&metrics.ops, 0)
		atomic.StoreInt64(&metrics.errors, 0)
		atomic.StoreInt64(&metrics.bytes, 0)
		metrics.mu.Lock()
		metrics.latencies = metrics.latencies[:0]
		metrics.mu.Unlock()
	}

	// Main benchmark
	benchCtx, cancel := context.WithTimeout(ctx, cfg.Duration)
	defer cancel()

	startTime := time.Now()
	r.runWorkers(benchCtx, target, cfg, metrics, r.ingestOperation)
	actualDuration := time.Since(startTime)

	// Calculate results
	result := r.calculateResult("ingest", actualDuration, metrics)

	r.mu.Lock()
	r.results["ingest"] = result
	r.mu.Unlock()

	r.logResult(result)
	return result, nil
}

// RunQueryBenchmark benchmarks query latency and throughput
func (r *Runner) RunQueryBenchmark(ctx context.Context, target ServiceTarget, cfg Config) (*Result, error) {
	r.logger.Info("starting query benchmark",
		"target", target.Name,
		"duration", cfg.Duration,
		"concurrency", cfg.Concurrency,
	)

	metrics := &Metrics{
		latencies: make([]time.Duration, 0, 10000),
	}

	// Warm-up phase
	if cfg.WarmUp > 0 {
		r.logger.Info("warm-up phase", "duration", cfg.WarmUp)
		warmCtx, cancel := context.WithTimeout(ctx, cfg.WarmUp)
		r.runWorkers(warmCtx, target, cfg, metrics, r.queryOperation)
		cancel()
		r.logger.Info("warm-up complete")

		// Reset metrics
		atomic.StoreInt64(&metrics.ops, 0)
		atomic.StoreInt64(&metrics.errors, 0)
		atomic.StoreInt64(&metrics.bytes, 0)
		metrics.mu.Lock()
		metrics.latencies = metrics.latencies[:0]
		metrics.mu.Unlock()
	}

	// Main benchmark
	benchCtx, cancel := context.WithTimeout(ctx, cfg.Duration)
	defer cancel()

	startTime := time.Now()
	r.runWorkers(benchCtx, target, cfg, metrics, r.queryOperation)
	actualDuration := time.Since(startTime)

	result := r.calculateResult("query", actualDuration, metrics)

	r.mu.Lock()
	r.results["query"] = result
	r.mu.Unlock()

	r.logResult(result)
	return result, nil
}

// RunLoadBenchmark runs a sustained load test
func (r *Runner) RunLoadBenchmark(ctx context.Context, target ServiceTarget, cfg Config) (*Result, error) {
	r.logger.Info("starting load benchmark",
		"target", target.Name,
		"duration", cfg.Duration,
		"concurrency", cfg.Concurrency,
	)

	metrics := &Metrics{
		latencies: make([]time.Duration, 0, 100000),
	}

	benchCtx, cancel := context.WithTimeout(ctx, cfg.Duration)
	defer cancel()

	// Determine operation based on target
	operation := r.queryOperation
	if target.Operation == "ingest" {
		operation = r.ingestOperation
	}

	startTime := time.Now()
	r.runWorkers(benchCtx, target, cfg, metrics, operation)
	actualDuration := time.Since(startTime)

	result := r.calculateResult("load", actualDuration, metrics)

	r.mu.Lock()
	r.results["load"] = result
	r.mu.Unlock()

	r.logResult(result)
	return result, nil
}

// GetResults returns all benchmark results
func (r *Runner) GetResults() map[string]*Result {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]*Result)
	for k, v := range r.results {
		results[k] = v
	}
	return results
}

// GetResult returns a specific benchmark result
func (r *Runner) GetResult(name string) (*Result, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result, ok := r.results[name]
	return result, ok
}

// runWorkers spawns concurrent workers
func (r *Runner) runWorkers(ctx context.Context, target ServiceTarget, cfg Config, metrics *Metrics, operation func(context.Context, ServiceTarget, *Metrics)) {
	var wg sync.WaitGroup

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			operation(ctx, target, metrics)
		}(i)
	}

	wg.Wait()
}

// ingestOperation simulates CSV ingestion
func (r *Runner) ingestOperation(ctx context.Context, target ServiceTarget, metrics *Metrics) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()

			// Simulate ingestion operation
			// In real implementation, would write CSV data to service
			simulatedBytes := int64(1024 + (start.UnixNano() % 4096)) // 1KB - 5KB
			simulatedLatency := time.Duration(1+start.UnixNano()%50) * time.Millisecond

			time.Sleep(simulatedLatency)

			atomic.AddInt64(&metrics.ops, 1)
			atomic.AddInt64(&metrics.bytes, simulatedBytes)

			latency := time.Since(start)
			metrics.mu.Lock()
			metrics.latencies = append(metrics.latencies, latency)
			metrics.mu.Unlock()
		}
	}
}

// queryOperation simulates API queries
func (r *Runner) queryOperation(ctx context.Context, target ServiceTarget, metrics *Metrics) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()

			// Simulate query operation
			// In real implementation, would make HTTP request to API
			simulatedBytes := int64(256 + (start.UnixNano() % 1024)) // 256B - 1.25KB response
			simulatedLatency := time.Duration(1+start.UnixNano()%20) * time.Millisecond

			time.Sleep(simulatedLatency)

			atomic.AddInt64(&metrics.ops, 1)
			atomic.AddInt64(&metrics.bytes, simulatedBytes)

			latency := time.Since(start)
			metrics.mu.Lock()
			metrics.latencies = append(metrics.latencies, latency)
			metrics.mu.Unlock()
		}
	}
}

// calculateResult computes final benchmark results
func (r *Runner) calculateResult(name string, duration time.Duration, metrics *Metrics) *Result {
	totalOps := atomic.LoadInt64(&metrics.ops)
	totalBytes := atomic.LoadInt64(&metrics.bytes)
	errors := atomic.LoadInt64(&metrics.errors)

	throughput := float64(totalOps) / duration.Seconds()

	result := &Result{
		Name:       name,
		Duration:   duration,
		TotalOps:   totalOps,
		TotalBytes: totalBytes,
		Errors:     errors,
		Throughput: throughput,
		Timestamp:  time.Now(),
	}

	// Calculate percentiles
	metrics.mu.Lock()
	if len(metrics.latencies) > 0 {
		result.Latencies = make([]time.Duration, len(metrics.latencies))
		copy(result.Latencies, metrics.latencies)
		result.p50 = r.calculatePercentile(result.Latencies, 0.50)
		result.p95 = r.calculatePercentile(result.Latencies, 0.95)
		result.p99 = r.calculatePercentile(result.Latencies, 0.99)
	}
	metrics.mu.Unlock()

	return result
}

// calculatePercentile calculates percentile from latency slice
func (r *Runner) calculatePercentile(latencies []time.Duration, p float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Simple implementation - sort and pick index
	// In production, use a more efficient algorithm
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	// Bubble sort for simplicity (for small datasets)
	n := len(sorted)
	for i := 0; i < n; i++ {
		for j := 0; j < n-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	index := int(math.Floor(p * float64(len(sorted)-1)))
	return sorted[index]
}

// logResult logs benchmark results
func (r *Runner) logResult(result *Result) {
	throughputMB := float64(result.TotalBytes) / (1024 * 1024) / result.Duration.Seconds()

	r.logger.Info("=== benchmark results ===", "name", result.Name)
	r.logger.Info("throughput",
		"ops/sec", fmt.Sprintf("%.2f", result.Throughput),
		"MB/sec", fmt.Sprintf("%.2f", throughputMB),
		"total_ops", result.TotalOps,
	)
	r.logger.Info("latency",
		"p50", result.p50,
		"p95", result.p95,
		"p99", result.p99,
	)
	r.logger.Info("errors", "count", result.Errors)
	r.logger.Info("duration", "actual", result.Duration)
}

// Percentile methods for external access
func (r *Result) P50() time.Duration { return r.p50 }
func (r *Result) P95() time.Duration { return r.p95 }
func (r *Result) P99() time.Duration { return r.p99 }

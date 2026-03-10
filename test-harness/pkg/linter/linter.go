// Package linter provides C code linting capabilities
package linter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Linter runs static analysis on C code
type Linter struct {
	servicesPath string
	sharedPath   string
}

// Result holds linting results for a file
type Result struct {
	File         string
	Errors       []Issue
	Warnings     []Issue
	StyleIssues  []Issue
	TotalIssues  int
	ErrorCount   int
	WarningCount int
	StyleCount   int
	Passed       bool
}

// Issue represents a single linting issue
type Issue struct {
	Line     int
	Column   int
	Severity string // error, warning, style
	Message  string
	Checker  string // cppcheck, gcc, etc.
}

// Summary aggregates results across all files
type Summary struct {
	Files         []Result
	TotalFiles    int
	TotalErrors   int
	TotalWarnings int
	TotalStyle    int
	TotalIssues   int
	Score         int    // 0-100
	Grade         string // A, B, C, D, F
	Passed        bool
}

// New creates a new Linter instance
func New(servicesPath, sharedPath string) *Linter {
	return &Linter{
		servicesPath: servicesPath,
		sharedPath:   sharedPath,
	}
}

// RunAll runs all available linters on all C files
func (l *Linter) RunAll() (*Summary, error) {
	summary := &Summary{
		Files: make([]Result, 0),
	}

	// Find all C files
	cFiles := l.findCFiles()
	summary.TotalFiles = len(cFiles)

	// Run cppcheck on each file
	for _, file := range cFiles {
		result, err := l.runCppcheck(file)
		if err != nil {
			// Log error but continue
			continue
		}
		summary.Files = append(summary.Files, *result)
		summary.TotalErrors += result.ErrorCount
		summary.TotalWarnings += result.WarningCount
		summary.TotalStyle += result.StyleCount
		summary.TotalIssues += result.TotalIssues
	}

	// Calculate score and grade
	summary.calculateScore()

	return summary, nil
}

// RunOnService runs linters on a specific service
func (l *Linter) RunOnService(serviceName string) (*Result, error) {
	servicePath := filepath.Join(l.servicesPath, serviceName)
	cFiles := l.findCFilesInDir(servicePath)

	if len(cFiles) == 0 {
		return nil, fmt.Errorf("no C files found in service %s", serviceName)
	}

	// Combine results from all files in service
	combinedResult := &Result{
		File: serviceName,
	}

	for _, file := range cFiles {
		result, err := l.runCppcheck(file)
		if err != nil {
			continue
		}
		combinedResult.Errors = append(combinedResult.Errors, result.Errors...)
		combinedResult.Warnings = append(combinedResult.Warnings, result.Warnings...)
		combinedResult.StyleIssues = append(combinedResult.StyleIssues, result.StyleIssues...)
		combinedResult.TotalIssues += result.TotalIssues
		combinedResult.ErrorCount += result.ErrorCount
		combinedResult.WarningCount += result.WarningCount
		combinedResult.StyleCount += result.StyleCount
	}

	combinedResult.Passed = combinedResult.ErrorCount == 0

	return combinedResult, nil
}

// findCFiles finds all C files in the services directory
func (l *Linter) findCFiles() []string {
	var files []string

	// Check each service directory
	services := []string{"ingestion", "aggregation", "query", "discovery", "cli"}
	for _, service := range services {
		servicePath := filepath.Join(l.servicesPath, service)
		files = append(files, l.findCFilesInDir(servicePath)...)
	}

	// Check shared library
	if l.sharedPath != "" {
		sharedSrc := filepath.Join(l.sharedPath, "src")
		files = append(files, l.findCFilesInDir(sharedSrc)...)
	}

	return files
}

// findCFilesInDir finds C files in a specific directory
func (l *Linter) findCFilesInDir(dir string) []string {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".c") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files
}

// runCppcheck runs cppcheck on a single file
func (l *Linter) runCppcheck(filePath string) (*Result, error) {
	result := &Result{
		File:        filePath,
		Errors:      make([]Issue, 0),
		Warnings:    make([]Issue, 0),
		StyleIssues: make([]Issue, 0),
	}

	// Build include paths
	includes := []string{
		"-I", filepath.Join(l.servicesPath, "shared", "include"),
	}

	// Run cppcheck
	cmd := exec.Command("cppcheck",
		"--enable=all",
		"--inconclusive",
		"--std=c99",
		"--suppress=missingIncludeSystem",
		"--suppress=unusedFunction",
		"--suppress=toomanyconfigs",
		"--template={file}:{line}:{column}:{severity}:{message}",
	)
	cmd.Args = append(cmd.Args, includes...)
	cmd.Args = append(cmd.Args, filePath)

	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		// cppcheck returns non-zero when issues found
		return result, nil
	}

	// Parse output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}

		issue := l.parseCppcheckLine(line)
		if issue != nil {
			switch issue.Severity {
			case "error":
				result.Errors = append(result.Errors, *issue)
				result.ErrorCount++
			case "warning":
				result.Warnings = append(result.Warnings, *issue)
				result.WarningCount++
			case "style", "performance", "portability":
				result.StyleIssues = append(result.StyleIssues, *issue)
				result.StyleCount++
			}
			result.TotalIssues++
		}
	}

	result.Passed = result.ErrorCount == 0

	return result, nil
}

// parseCppcheckLine parses a cppcheck output line
func (l *Linter) parseCppcheckLine(line string) *Issue {
	// Format: file:line:column:severity:message
	// Example: ingestion.c:174:22:warning:Width 11 given in format string

	parts := strings.SplitN(line, ":", 5)
	if len(parts) < 4 {
		return nil
	}

	issue := &Issue{
		Checker: "cppcheck",
	}

	// Parse line number
	fmt.Sscanf(parts[1], "%d", &issue.Line)
	// Parse column
	fmt.Sscanf(parts[2], "%d", &issue.Column)
	// Severity
	issue.Severity = parts[3]
	// Message
	if len(parts) >= 5 {
		issue.Message = parts[4]
	}

	return issue
}

// RunGCCChecks runs gcc with strict warning flags
func (l *Linter) RunGCCChecks(serviceName string) (*Result, error) {
	servicePath := filepath.Join(l.servicesPath, serviceName)
	cFile := filepath.Join(servicePath, serviceName+".c")

	result := &Result{
		File:        cFile,
		Errors:      make([]Issue, 0),
		Warnings:    make([]Issue, 0),
		StyleIssues: make([]Issue, 0),
	}

	// Check if file exists
	if _, err := os.Stat(cFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("source file not found: %s", cFile)
	}

	// Run gcc with strict warnings
	includes := "-I" + filepath.Join(l.servicesPath, "shared", "include")
	cmd := exec.Command("gcc",
		"-c",
		"-Wall",
		"-Wextra",
		"-Wpedantic",
		"-Wshadow",
		"-Wstrict-overflow",
		"-Wstrict-prototypes",
		"-Wmissing-prototypes",
		"-Wmissing-declarations",
		"-Wcast-align",
		"-Wwrite-strings",
		"-Werror=implicit-function-declaration",
		"-Werror=return-type",
		includes,
		cFile,
		"-o", "/dev/null",
	)

	output, err := cmd.CombinedOutput()
	// gcc returns non-zero if there are errors

	// Parse gcc output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}

		issue := l.parseGCCLine(line)
		if issue != nil {
			if issue.Severity == "error" {
				result.Errors = append(result.Errors, *issue)
				result.ErrorCount++
			} else {
				result.Warnings = append(result.Warnings, *issue)
				result.WarningCount++
			}
			result.TotalIssues++
		}
	}

	result.Passed = result.ErrorCount == 0 && err == nil

	return result, nil
}

// parseGCCLine parses a gcc warning/error line
func (l *Linter) parseGCCLine(line string) *Issue {
	// Format: file:line:column: severity: message
	// Example: ingestion.c:174:22: warning: ...

	// Match gcc output pattern
	re := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(warning|error|note):\s*(.+)$`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 6 {
		return nil
	}

	issue := &Issue{
		Checker: "gcc",
	}

	fmt.Sscanf(matches[2], "%d", &issue.Line)
	fmt.Sscanf(matches[3], "%d", &issue.Column)
	issue.Severity = matches[4]
	issue.Message = matches[5]

	return issue
}

// calculateScore computes the linting score based on issues found
func (s *Summary) calculateScore() {
	if s.TotalFiles == 0 {
		s.Score = 0
		s.Grade = "F"
		s.Passed = false
		return
	}

	// Scoring formula:
	// Base score: 100
	// -5 points per error
	// -2 points per warning
	// -0.5 points per style issue
	// Minimum score: 0

	baseScore := 100.0
	deductions := float64(s.TotalErrors)*5.0 +
		float64(s.TotalWarnings)*2.0 +
		float64(s.TotalStyle)*0.5

	score := baseScore - deductions
	if score < 0 {
		score = 0
	}

	s.Score = int(score)

	// Assign grade
	switch {
	case s.Score >= 93:
		s.Grade = "A"
		s.Passed = true
	case s.Score >= 90:
		s.Grade = "A-"
		s.Passed = true
	case s.Score >= 87:
		s.Grade = "B+"
		s.Passed = true
	case s.Score >= 83:
		s.Grade = "B"
		s.Passed = true
	case s.Score >= 80:
		s.Grade = "B-"
		s.Passed = true
	case s.Score >= 77:
		s.Grade = "C+"
	case s.Score >= 73:
		s.Grade = "C"
	case s.Score >= 70:
		s.Grade = "C-"
	case s.Score >= 60:
		s.Grade = "D"
	default:
		s.Grade = "F"
		s.Passed = false
	}
}

// PrintSummary prints a formatted summary of linting results
func (s *Summary) PrintSummary() {
	fmt.Println("\n=== C Code Linting Results ===")
	fmt.Printf("Files Analyzed: %d\n", s.TotalFiles)
	fmt.Printf("Total Issues: %d\n", s.TotalIssues)
	fmt.Printf("  Errors: %d\n", s.TotalErrors)
	fmt.Printf("  Warnings: %d\n", s.TotalWarnings)
	fmt.Printf("  Style Issues: %d\n", s.TotalStyle)
	fmt.Printf("Score: %d/100 (Grade: %s)\n", s.Score, s.Grade)

	if s.Passed {
		fmt.Println("Status: ✓ PASSED")
	} else {
		fmt.Println("Status: ✗ FAILED")
	}

	// Print issues by file
	if s.TotalIssues > 0 {
		fmt.Println("\nIssues by File:")
		for _, file := range s.Files {
			if file.TotalIssues > 0 {
				fmt.Printf("\n%s:\n", file.File)
				for _, issue := range file.Errors {
					fmt.Printf("  [ERROR] Line %d: %s\n", issue.Line, issue.Message)
				}
				for _, issue := range file.Warnings {
					fmt.Printf("  [WARNING] Line %d: %s\n", issue.Line, issue.Message)
				}
				if file.StyleCount > 0 {
					fmt.Printf("  ... and %d style issues\n", file.StyleCount)
				}
			}
		}
	}
}

// IsCppcheckAvailable checks if cppcheck is installed
func IsCppcheckAvailable() bool {
	_, err := exec.LookPath("cppcheck")
	return err == nil
}

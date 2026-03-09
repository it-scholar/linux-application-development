package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"weather-station-test/pkg/ci"
	"weather-station-test/pkg/github"
	"weather-station-test/pkg/gitlab"
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "run full ci pipeline",
	Long: `executes the full ci/cd pipeline for continuous integration.

pipeline steps:
  1. validate service contracts
  2. run all test suites
  3. execute performance benchmarks
  4. run chaos tests
  5. calculate final grade
  6. post results to pr/mr

supported platforms:
  - github actions
  - gitlab ci
  - local ci mode

examples:
  # run ci pipeline (github)
  test-harness ci --github-token $token --fail-threshold 80

  # run ci pipeline (gitlab)
  test-harness ci --gitlab-token $token --fail-threshold 80

  # local ci mode
  test-harness ci --fail-threshold 80`,
	RunE: runCI,
}

var ciFlags struct {
	githubToken   string
	gitlabToken   string
	failThreshold int
	prNumber      int
	repo          string
	mrNumber      int
	gitLabProject string
}

func init() {
	rootCmd.AddCommand(ciCmd)

	ciCmd.Flags().StringVar(&ciFlags.githubToken, "github-token", "", "github token for pr comments")
	ciCmd.Flags().StringVar(&ciFlags.gitlabToken, "gitlab-token", "", "gitlab token for mr comments")
	ciCmd.Flags().IntVar(&ciFlags.failThreshold, "fail-threshold", 80, "minimum score to pass")
	ciCmd.Flags().IntVar(&ciFlags.prNumber, "pr-number", 0, "pull request number")
	ciCmd.Flags().StringVar(&ciFlags.repo, "repo", "", "repository (owner/repo)")
	ciCmd.Flags().IntVar(&ciFlags.mrNumber, "mr-number", 0, "merge request number")
	ciCmd.Flags().StringVar(&ciFlags.gitLabProject, "gitlab-project", "", "gitlab project path")
}

func runCI(cmd *cobra.Command, args []string) error {
	logger.Info("running ci pipeline", "fail_threshold", ciFlags.failThreshold)

	// Detect CI configuration from environment or flags
	config := ci.DetectConfig()

	// Override with command-line flags
	if ciFlags.githubToken != "" {
		config.Platform = ci.PlatformGitHub
		config.Token = ciFlags.githubToken
		config.Repository = ciFlags.repo
		config.PullRequestID = ciFlags.prNumber
	} else if ciFlags.gitlabToken != "" {
		config.Platform = ci.PlatformGitLab
		config.Token = ciFlags.gitlabToken
		config.Repository = ciFlags.gitLabProject
		config.PullRequestID = ciFlags.mrNumber
	}

	// Log platform info
	switch config.Platform {
	case ci.PlatformGitHub:
		logger.Info("platform", "name", "github actions")
		logger.Info("repository", "repo", config.Repository)
		logger.Info("commit", "sha", config.CommitSHA[:8])
		if config.PullRequestID > 0 {
			logger.Info("pull request", "pr", config.PullRequestID)
		}
	case ci.PlatformGitLab:
		logger.Info("platform", "name", "gitlab ci")
		logger.Info("project", "path", config.Repository)
		logger.Info("commit", "sha", config.CommitSHA[:8])
		if config.PullRequestID > 0 {
			logger.Info("merge request", "mr", config.PullRequestID)
		}
	default:
		logger.Info("platform", "name", "local ci mode")
	}

	// Create CI client
	client := ci.NewClient(config)

	// Run the pipeline
	ctx := context.Background()
	result, err := client.RunPipeline(ctx)
	if err != nil {
		logger.Error("pipeline failed", "error", err)
		os.Exit(1)
	}

	// Log results
	logger.Info("=== ci pipeline results ===")
	logger.Info("duration", "value", result.Duration)
	logger.Info("tests", "passed", result.TestResults.PassedTests,
		"failed", result.TestResults.FailedTests,
		"total", result.TestResults.TotalTests)

	if result.Grade != nil {
		logger.Info("grade", "score", result.Grade.Score,
			"max", result.Grade.MaxScore,
			"percentage", fmt.Sprintf("%.1f%%", result.Grade.Percentage),
			"letter", result.Grade.LetterGrade)
	}

	// Post results to platform if configured
	if config.Platform == ci.PlatformGitHub && config.PullRequestID > 0 && config.Token != "" {
		ghClient := github.NewClient(github.Config{
			Token:      config.Token,
			Repository: config.Repository,
		})

		testResults := &github.TestResults{
			TotalTests:   result.TestResults.TotalTests,
			PassedTests:  result.TestResults.PassedTests,
			FailedTests:  result.TestResults.FailedTests,
			SkippedTests: result.TestResults.SkippedTests,
			Duration:     result.Duration,
			Score:        result.Grade.Score,
			MaxScore:     result.Grade.MaxScore,
			Grade:        result.Grade.LetterGrade,
		}

		if err := ghClient.PostTestResults(ctx, config.PullRequestID, testResults); err != nil {
			logger.Error("failed to post github comment", "error", err)
		} else {
			logger.Info("posted results to github pr", "pr", config.PullRequestID)
		}
	}

	if config.Platform == ci.PlatformGitLab && config.PullRequestID > 0 && config.Token != "" {
		glClient := gitlab.NewClient(gitlab.Config{
			Token:   config.Token,
			Project: config.Repository,
			APIURL:  config.APIURL,
		})

		testResults := &gitlab.TestResults{
			TotalTests:   result.TestResults.TotalTests,
			PassedTests:  result.TestResults.PassedTests,
			FailedTests:  result.TestResults.FailedTests,
			SkippedTests: result.TestResults.SkippedTests,
			Duration:     result.Duration,
			Score:        result.Grade.Score,
			MaxScore:     result.Grade.MaxScore,
			Grade:        result.Grade.LetterGrade,
		}

		if err := glClient.PostTestResults(ctx, config.PullRequestID, testResults); err != nil {
			logger.Error("failed to post gitlab comment", "error", err)
		} else {
			logger.Info("posted results to gitlab mr", "mr", config.PullRequestID)
		}
	}

	// Check threshold
	if result.Grade != nil && result.Grade.Percentage >= float64(ciFlags.failThreshold) {
		logger.Info("✓ ci pipeline passed!")
		return nil
	}

	logger.Error("✗ ci pipeline failed: score below threshold")
	os.Exit(1)
	return nil
}

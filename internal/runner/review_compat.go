package runner

import "github.com/saaga0h/themis/internal/review"

// blockingThreshold, ReviewFinding, ReviewResults, and the four review-results
// functions are defined in internal/review/. These aliases let runner-package
// tests reference them without a qualifier while keeping runner.go clean.

const blockingThreshold = review.BlockingThreshold

type ReviewFinding = review.ReviewFinding
type ReviewResults = review.ReviewResults

var readReviewResults = review.ReadReviewResults
var countFindingsBySeverity = review.CountFindingsBySeverity
var determineBlockingStatus = review.DetermineBlockingStatus
var formatBlockingFindings = review.FormatBlockingFindings

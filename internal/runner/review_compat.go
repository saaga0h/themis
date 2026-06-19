package runner

import "github.com/saaga0h/themis/internal/review"

// Unqualified aliases so package-runner tests can reference review types and
// the two helper functions they call directly without importing internal/review.
const blockingThreshold = review.BlockingThreshold

type ReviewFinding = review.ReviewFinding
type ReviewResults = review.ReviewResults

var determineBlockingStatus = review.DetermineBlockingStatus
var formatBlockingFindings = review.FormatBlockingFindings

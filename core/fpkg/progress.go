package fpkg

// ProgressReporter reports build progress as a percentage (0–100) and phase label.
type ProgressReporter func(percent float64, phase string)

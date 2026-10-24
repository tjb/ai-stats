package roi

import "math"

const minutesPerAcceptedLine = 2

// Result holds calculated ROI values.
type Result struct {
	MinutesSaved     float64 `json:"minutes_saved"`
	HoursSaved       float64 `json:"hours_saved"`
	DollarValueSaved float64 `json:"dollar_value_saved"`
}

// Calculate returns the ROI for the given lines accepted and hourly rate.
func Calculate(linesAccepted int, hourlyRate int) Result {
	minutes := float64(linesAccepted) * minutesPerAcceptedLine
	hours := minutes / 60
	dollars := hours * float64(hourlyRate)
	return Result{
		MinutesSaved:     math.Round(minutes*10) / 10,
		HoursSaved:       math.Round(hours*10) / 10,
		DollarValueSaved: math.Round(dollars),
	}
}

// AcceptanceRate computes percentage (0–100) rounded to 1 decimal.
func AcceptanceRate(acceptances, suggestions int) float64 {
	if suggestions == 0 {
		return 0
	}
	rate := float64(acceptances) / float64(suggestions) * 100
	return math.Round(rate*10) / 10
}

// PctChange computes period-over-period change as an integer percentage.
func PctChange(current, previous int) int {
	if previous == 0 {
		return 0
	}
	return int(math.Round(float64(current-previous) / float64(previous) * 100))
}

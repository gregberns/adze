package ui

// SuccessSymbol returns the success symbol (checkmark), colored green when
// color is enabled.
func SuccessSymbol(color bool) string {
	return Colorize("✓", ColorGreen, color)
}

// FailureSymbol returns the failure symbol (cross), colored red when color
// is enabled.
func FailureSymbol(color bool) string {
	return Colorize("✗", ColorRed, color)
}

// WarningSymbol returns the warning symbol, colored yellow when color is
// enabled.
func WarningSymbol(color bool) string {
	return Colorize("⚠", ColorYellow, color)
}

// SkipSymbol returns the skip symbol (dash), colored yellow when color is
// enabled.
func SkipSymbol(color bool) string {
	return Colorize("-", ColorYellow, color)
}

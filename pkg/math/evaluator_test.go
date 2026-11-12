package math

import (
	"strings"
	"testing"
)

func TestEvaluator(t *testing.T) {
	ev := NewEvaluator()

	tests := []struct {
		input    string
		expected string
	}{
		// Basic operations
		{"2 + 2", "4"},
		{"10 - 3", "7"},
		{"6 * 7", "42"},
		{"100 / 4", "25"},
		{"2 + 3 * 4", "14"},
		{"(2 + 3) * 4", "20"},
		{"-5 + 10", "5"},
		{"10 + -5", "5"},

		// Floating point & decimal formatting
		{"3.5 + 2.25", "5.75"},
		{"1 / 2", "0.5"},
		{"1 / 4", "0.25"},

		// Exponents
		{"2 ^ 3", "8"},
		{"3 ^ 2", "9"},

		// Modulo
		{"10 mod 3", "1"},
		{"14 mod 5", "4"},

		// Functions
		{"sqrt(16)", "4"},
		{"sqrt(25)", "5"},
		{"abs(-42)", "42"},
		{"round(3.7)", "4"},
		{"floor(3.7)", "3"},
		{"ceil(3.2)", "4"},

		// Percentage operations
		{"25%", "0.25"},
		{"200 * 15%", "30"},
		{"50 + 10%", "55"},
		{"100 - 25%", "75"},
		{"100 / 50%", "200"},

		// Nested expressions
		{"((2 + 3) * (4 + 6)) / 2", "25"},
	}

	for _, tc := range tests {
		result := ev.Evaluate(tc.input)
		if result != tc.expected {
			t.Errorf("Evaluate(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestEvaluatorErrors(t *testing.T) {
	ev := NewEvaluator()

	errorTests := []struct {
		input       string
		errContains string
	}{
		{"10 / 0", "Division by zero"},
		{"sqrt(-1)", "negative number"},
		{"(2 + 3", "unmatched opening parenthesis"},
		{"2 + * 3", "syntax error"},
		{"@#$%", "unrecognized character"},
	}

	for _, tc := range errorTests {
		result := ev.Evaluate(tc.input)
		if !strings.Contains(strings.ToLower(result), strings.ToLower(tc.errContains)) {
			t.Errorf("Evaluate(%q) = %q, should contain %q", tc.input, result, tc.errContains)
		}
	}
}

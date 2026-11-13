package order

import (
	"testing"
)

func TestPassesLuhn(t *testing.T) {
	for _, test := range []struct {
		number string
		valid  bool
	}{
		{"12345678903", true},
		{"79927398713", true},
		{"00012345678903", true},
		{"12345678904", false},
		{"", false},
		{"123abc", false},
		{"-12345678903", false},
		{"１２３", false},
	} {
		if got := PassesLuhn(test.number); got != test.valid {
			t.Errorf("PassesLuhn(%q) = %v, want %v", test.number, got, test.valid)
		}
	}
}

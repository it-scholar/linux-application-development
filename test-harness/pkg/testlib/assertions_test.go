package testlib

import (
	"errors"
	"testing"
)

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		wantPass bool
	}{
		{"equal ints", 42, 42, true},
		{"different ints", 42, 43, false},
		{"equal strings", "hello", "hello", true},
		{"different strings", "hello", "world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Equal(t, tt.expected, tt.actual)
			if result != tt.wantPass {
				t.Errorf("Equal(%v, %v) = %v, want %v",
					tt.expected, tt.actual, result, tt.wantPass)
			}
		})
	}
}

func TestNotEqual(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		wantPass bool
	}{
		{"different values", 42, 43, true},
		{"equal values", 42, 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NotEqual(t, tt.expected, tt.actual)
			if result != tt.wantPass {
				t.Errorf("NotEqual(%v, %v) = %v, want %v",
					tt.expected, tt.actual, result, tt.wantPass)
			}
		})
	}
}

func TestTrue(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		wantPass bool
	}{
		{"true passes", true, true},
		{"false fails", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := True(t, tt.value)
			if result != tt.wantPass {
				t.Errorf("True(%v) = %v, want %v", tt.value, result, tt.wantPass)
			}
		})
	}
}

func TestFalse(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		wantPass bool
	}{
		{"false passes", false, true},
		{"true fails", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := False(t, tt.value)
			if result != tt.wantPass {
				t.Errorf("False(%v) = %v, want %v", tt.value, result, tt.wantPass)
			}
		})
	}
}

func TestNil(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		wantPass bool
	}{
		{"nil passes", nil, true},
		{"non-nil fails", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Nil(t, tt.value)
			if result != tt.wantPass {
				t.Errorf("Nil(%v) = %v, want %v", tt.value, result, tt.wantPass)
			}
		})
	}
}

func TestNotNil(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		wantPass bool
	}{
		{"non-nil passes", "hello", true},
		{"nil fails", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NotNil(t, tt.value)
			if result != tt.wantPass {
				t.Errorf("NotNil(%v) = %v, want %v", tt.value, result, tt.wantPass)
			}
		})
	}
}

func TestNoError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantPass bool
	}{
		{"nil error passes", nil, true},
		{"non-nil error fails", errors.New("test error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NoError(t, tt.err)
			if result != tt.wantPass {
				t.Errorf("NoError(%v) = %v, want %v", tt.err, result, tt.wantPass)
			}
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantPass bool
	}{
		{"non-nil error passes", errors.New("test error"), true},
		{"nil error fails", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Error(t, tt.err)
			if result != tt.wantPass {
				t.Errorf("Error(%v) = %v, want %v", tt.err, result, tt.wantPass)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		wantPass bool
	}{
		{"contains substring", "hello world", "world", true},
		{"does not contain", "hello world", "foo", false},
		{"empty substring", "hello", "", true},
		{"same string", "hello", "hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(t, tt.s, tt.substr)
			if result != tt.wantPass {
				t.Errorf("Contains(%q, %q) = %v, want %v",
					tt.s, tt.substr, result, tt.wantPass)
			}
		})
	}
}

func TestGreater(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		wantPass bool
	}{
		{"greater passes", 10, 5, true},
		{"equal fails", 5, 5, false},
		{"less fails", 5, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Greater(t, tt.a, tt.b)
			if result != tt.wantPass {
				t.Errorf("Greater(%d, %d) = %v, want %v",
					tt.a, tt.b, result, tt.wantPass)
			}
		})
	}
}

func TestLess(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		wantPass bool
	}{
		{"less passes", 5, 10, true},
		{"equal fails", 5, 5, false},
		{"greater fails", 10, 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Less(t, tt.a, tt.b)
			if result != tt.wantPass {
				t.Errorf("Less(%d, %d) = %v, want %v",
					tt.a, tt.b, result, tt.wantPass)
			}
		})
	}
}

func TestErrorContains(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		substr   string
		wantPass bool
	}{
		{"error contains substring", errors.New("test error message"), "error", true},
		{"error does not contain", errors.New("test error"), "not found", false},
		{"nil error fails", nil, "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrorContains(t, tt.err, tt.substr)
			if result != tt.wantPass {
				t.Errorf("ErrorContains(%v, %q) = %v, want %v",
					tt.err, tt.substr, result, tt.wantPass)
			}
		})
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		prefix   string
		wantPass bool
	}{
		{"has prefix", "hello world", "hello", true},
		{"does not have prefix", "hello world", "world", false},
		{"empty prefix", "hello", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPrefix(t, tt.s, tt.prefix)
			if result != tt.wantPass {
				t.Errorf("HasPrefix(%q, %q) = %v, want %v",
					tt.s, tt.prefix, result, tt.wantPass)
			}
		})
	}
}

func TestHasSuffix(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		suffix   string
		wantPass bool
	}{
		{"has suffix", "hello world", "world", true},
		{"does not have suffix", "hello world", "hello", false},
		{"empty suffix", "hello", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasSuffix(t, tt.s, tt.suffix)
			if result != tt.wantPass {
				t.Errorf("HasSuffix(%q, %q) = %v, want %v",
					tt.s, tt.suffix, result, tt.wantPass)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantVal float64
		wantOk  bool
	}{
		{"int", 42, 42.0, true},
		{"int8", int8(42), 42.0, true},
		{"int16", int16(42), 42.0, true},
		{"int32", int32(42), 42.0, true},
		{"int64", int64(42), 42.0, true},
		{"uint", uint(42), 42.0, true},
		{"float32", float32(42.5), 42.5, true},
		{"float64", 42.5, 42.5, true},
		{"string", "not a number", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := toFloat64(tt.value)
			if ok != tt.wantOk {
				t.Errorf("toFloat64(%v) ok = %v, want %v", tt.value, ok, tt.wantOk)
			}
			if ok && val != tt.wantVal {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.value, val, tt.wantVal)
			}
		})
	}
}

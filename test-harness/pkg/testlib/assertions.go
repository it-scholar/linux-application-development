package testlib

import (
	"reflect"
	"strings"
)

// testingt is a subset of testing.t for use outside of test files
type TestingT interface {
	Errorf(format string, args ...interface{})
	Fail()
	Failed() bool
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}

// assert equals two values
func Equal(t TestingT, expected, actual interface{}, msg ...string) bool {
	if reflect.DeepEqual(expected, actual) {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected %v but got %v", msgStr, expected, actual)
	return false
}

// assert not equals two values
func NotEqual(t TestingT, expected, actual interface{}, msg ...string) bool {
	if !reflect.DeepEqual(expected, actual) {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected values to be different but both are %v", msgStr, expected)
	return false
}

// assert true
func True(t TestingT, value bool, msg ...string) bool {
	if value {
		return true
	}

	msgStr := "expected true but got false"
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ")
	}

	t.Errorf(msgStr)
	return false
}

// assert false
func False(t TestingT, value bool, msg ...string) bool {
	if !value {
		return true
	}

	msgStr := "expected false but got true"
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ")
	}

	t.Errorf(msgStr)
	return false
}

// assert nil
func Nil(t TestingT, value interface{}, msg ...string) bool {
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil()) {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected nil but got %v", msgStr, value)
	return false
}

// assert not nil
func NotNil(t TestingT, value interface{}, msg ...string) bool {
	if value != nil && !(reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil()) {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected not nil but got nil", msgStr)
	return false
}

// assert contains substring
func Contains(t TestingT, s, substr string, msg ...string) bool {
	if strings.Contains(s, substr) {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected string to contain %q but it didn't\nstring: %q", msgStr, substr, s)
	return false
}

// assert string starts with prefix
func HasPrefix(t TestingT, s, prefix string, msg ...string) bool {
	if strings.HasPrefix(s, prefix) {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected string to start with %q but it didn't\nstring: %q", msgStr, prefix, s)
	return false
}

// assert string ends with suffix
func HasSuffix(t TestingT, s, suffix string, msg ...string) bool {
	if strings.HasSuffix(s, suffix) {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected string to end with %q but it didn't\nstring: %q", msgStr, suffix, s)
	return false
}

// assert greater than
func Greater(t TestingT, a, b interface{}, msg ...string) bool {
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)

	if !aOk || !bOk {
		t.Errorf("cannot compare non-numeric values: %v, %v", a, b)
		return false
	}

	if aFloat > bFloat {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected %v > %v", msgStr, a, b)
	return false
}

// assert less than
func Less(t TestingT, a, b interface{}, msg ...string) bool {
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)

	if !aOk || !bOk {
		t.Errorf("cannot compare non-numeric values: %v, %v", a, b)
		return false
	}

	if aFloat < bFloat {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected %v < %v", msgStr, a, b)
	return false
}

// assert no error
func NoError(t TestingT, err error, msg ...string) bool {
	if err == nil {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sunexpected error: %v", msgStr, err)
	return false
}

// assert error
func Error(t TestingT, err error, msg ...string) bool {
	if err != nil {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected error but got nil", msgStr)
	return false
}

// assert error contains
func ErrorContains(t TestingT, err error, substr string, msg ...string) bool {
	if err == nil {
		msgStr := ""
		if len(msg) > 0 {
			msgStr = strings.Join(msg, " ") + ": "
		}
		t.Errorf("%sexpected error containing %q but got nil", msgStr, substr)
		return false
	}

	if strings.Contains(err.Error(), substr) {
		return true
	}

	msgStr := ""
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ") + ": "
	}

	t.Errorf("%sexpected error containing %q but got: %v", msgStr, substr, err)
	return false
}

// helper to convert to float64
func toFloat64(v interface{}) (float64, bool) {
	switch i := v.(type) {
	case int:
		return float64(i), true
	case int8:
		return float64(i), true
	case int16:
		return float64(i), true
	case int32:
		return float64(i), true
	case int64:
		return float64(i), true
	case uint:
		return float64(i), true
	case uint8:
		return float64(i), true
	case uint16:
		return float64(i), true
	case uint32:
		return float64(i), true
	case uint64:
		return float64(i), true
	case float32:
		return float64(i), true
	case float64:
		return i, true
	default:
		return 0, false
	}
}

// fail immediately
func Fail(t TestingT, msg ...string) {
	msgStr := "test failed"
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ")
	}
	t.Errorf(msgStr)
	t.Fail()
}

// skip test
func Skip(t TestingT, msg ...string) {
	msgStr := "test skipped"
	if len(msg) > 0 {
		msgStr = strings.Join(msg, " ")
	}
	t.Log(msgStr)
}

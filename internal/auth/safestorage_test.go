package auth

import (
	"reflect"
	"runtime"
	"testing"
)

func TestDedupe(t *testing.T) {
	got := dedupe([]string{"a", "b", "a", "c", "b"})
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("dedupe = %v, want %v", got, want)
	}
	if got := dedupe(nil); got != nil {
		t.Errorf("dedupe(nil) = %v, want nil", got)
	}
}

func TestChromiumIterations(t *testing.T) {
	want := 1003
	if runtime.GOOS == "linux" {
		want = 1
	}
	if got := chromiumIterations(); got != want {
		t.Errorf("chromiumIterations on %s = %d, want %d", runtime.GOOS, got, want)
	}
}

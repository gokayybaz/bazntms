//go:build windows

package agent

import "testing"

func TestETWLayout(t *testing.T) {
	if err := checkLayout(); err != nil {
		t.Fatal(err)
	}
}

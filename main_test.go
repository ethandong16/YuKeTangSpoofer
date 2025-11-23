package main_test

import (
	main "YuKeTangSpoofer"
	"net/http"
	"testing"
)

func TestGetChapters(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		cid    string
		Cookie []*http.Cookie
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := main.GetChapters(tt.cid, tt.Cookie)
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("GetChapters() = %v, want %v", got, tt.want)
			}
		})
	}
}

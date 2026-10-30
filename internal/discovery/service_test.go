package discovery

import "testing"

func TestRunStatus(t *testing.T) {
	cases := []struct {
		ok, failed int
		want       string
	}{
		{5, 0, "success"},
		{0, 0, "success"},
		{0, 3, "error"},
		{4, 1, "partial"},
	}
	for _, tc := range cases {
		if got := runStatus(tc.ok, tc.failed); got != tc.want {
			t.Errorf("runStatus(%d, %d) = %s, quer %s", tc.ok, tc.failed, got, tc.want)
		}
	}
}

func TestMergeKeywords(t *testing.T) {
	got := mergeKeywords([]string{"Backend", " devops "}, []string{"backend", "", "RH"})
	want := []string{"backend", "devops", "rh"}
	if len(got) != len(want) {
		t.Fatalf("got %v, quer %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, quer %v", got, want)
		}
	}
}

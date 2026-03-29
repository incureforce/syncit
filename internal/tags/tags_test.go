package tags

import "testing"

func TestSubset(t *testing.T) {
	if !Subset(nil, nil) {
		t.Fatal()
	}
	if !Subset([]string{}, []string{"a"}) {
		t.Fatal()
	}
	if !Subset([]string{"a"}, []string{"a", "b"}) {
		t.Fatal()
	}
	if Subset([]string{"a"}, []string{"b"}) {
		t.Fatal()
	}
}

func TestEqual(t *testing.T) {
	if !Equal(nil, nil) || !Equal([]string{}, []string{}) {
		t.Fatal()
	}
	if !Equal([]string{"a", "b"}, []string{"a", "b"}) {
		t.Fatal()
	}
	if Equal([]string{"a"}, []string{"b"}) || Equal([]string{"a"}, []string{"a", "a"}) {
		t.Fatal()
	}
}

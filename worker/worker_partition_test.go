package worker

import "testing"

func TestReducerForKeyStable(t *testing.T) {
	nReduce := 8
	key := "same-key"
	first := reducerForKey(key, nReduce)
	for i := 0; i < 100; i++ {
		got := reducerForKey(key, nReduce)
		if got != first {
			t.Fatalf("expected stable reducer for key %q: %d != %d", key, got, first)
		}
	}
}

func TestReducerForKeyRange(t *testing.T) {
	nReduce := 7
	keys := []string{"a", "b", "c", "foo", "bar", "baz", "k1", "k2", "k3"}
	for _, key := range keys {
		got := reducerForKey(key, nReduce)
		if got < 0 || got >= nReduce {
			t.Fatalf("reducer id out of range for key %q: %d", key, got)
		}
	}
}

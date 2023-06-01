package testutil

func FakeFunc[Func any](mock Func, original *Func) (restore func()) {
	oldFunc := *original
	*original = mock
	return func() {
		*original = oldFunc
	}
}

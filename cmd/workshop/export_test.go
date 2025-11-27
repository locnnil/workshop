package main

var (
	MaybePresentWarnings  = maybePresentWarnings
	WriteWarningTimestamp = writeWarningTimestamp
)

func MockTextEditor(f func(inPath string, inContent []byte) ([]byte, error)) (restore func()) {
	old := runTextEditor
	runTextEditor = f
	return func() {
		runTextEditor = old
	}
}

package cmdutil

var (
	CanUnicode      = canUnicode
	ColorTable      = colorTable
	MonoColorTable  = mono
	ColorColorTable = color
	NoEscColorTable = noesc
)

func MockIsStdoutTTY(t bool) (restore func()) {
	oldIsStdoutTTY := isStdoutTTY
	isStdoutTTY = t
	return func() {
		isStdoutTTY = oldIsStdoutTTY
	}
}

package progress

type Reporter struct {
	Name   string
	Report func(label string, done, total int)
}

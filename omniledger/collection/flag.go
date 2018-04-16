package collection

type flag struct {
	value bool
}

// Methods

func (f *flag) Enable() {
	f.value = true
}

func (f *flag) Disable() {
	f.value = false
}

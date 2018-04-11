package collection

type flag struct {
	value bool
}

// Methods

func (this *flag) Enable() {
	this.value = true
}

func (this *flag) Disable() {
	this.value = false
}

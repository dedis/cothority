package sda

type TreeNodeInstance struct {
}

func (tni *TreeNodeInstance) Token() *Token {
	return &Token{}
}

func (tni *TreeNodeInstance) Shutdown() error {
	return nil
}

// legacy reasons
func (tni *TreeNodeInstance) Start() error {
	return nil
}

package randhound

func (p *ProtocolRandHound) Hash(bytes ...[]byte) []byte {
	h := p.ProtocolStruct.Host.Suite().Hash()
	for _, b := range bytes {
		h.Write(b)
	}
	return h.Sum(nil)
}

package personhood

import "math"

// score returns a value that can be used to sort the messages.
func (msg *Message) score() uint64 {
	return msg.Reward *
		uint64(1+math.Log2(float64(msg.Balance)/float64(msg.Reward)))
}

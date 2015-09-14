package sign

import "github.com/ineiti/cothorities/helpers/coconet"

// Functions used in collective signing
// That are direclty related to the generation/ verification/ sending
// of the Simple Combined Public Key Signature

// Send children challenges
func (sn *Node) SendChildrenChallenges(view int, chm *ChallengeMessage) error {
	for _, child := range sn.Children(view) {
		var messg coconet.BinaryMarshaler
		messg = &SigningMessage{View: view, Type: Challenge, Chm: chm}

		// fmt.Println(sn.Name(), "send to", i, child, "on view", view)
		if err := child.Put(messg); err != nil {
			return err
		}
	}

	return nil
}

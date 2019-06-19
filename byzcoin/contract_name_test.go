package byzcoin

import "testing"

func TestService_Naming(t *testing.T) {
	_ = newSer(t, 1, testInterval)

	// Create a value transaction that we will name.

	// FAIL - use a bad signature
	// s.service().ResolveInstanceID()

	// FAIL - use a use an instance that does not exist

	// FAIL - use a signer that is not authorized by the instance to spawn

	// FAIL - missing instance name

	// Make one or more names should succeed

	// Check that the names for a chain.
}

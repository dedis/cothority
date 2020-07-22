package bevm

// PROTOSTART
// package bevm;
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "BEvmProto";

// ViewCallRequest is a request to execute a view method (read-only).
type ViewCallRequest struct {
	ByzCoinID       []byte
	BEvmInstanceID  []byte
	AccountAddress  []byte
	ContractAddress []byte
	CallData        []byte
}

// ViewCallResponse is the response to ViewCallRequest, containing the method
// response.
type ViewCallResponse struct {
	Result []byte
}

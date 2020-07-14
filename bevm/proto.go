package bevm

// PROTOSTART
// package bevm;
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "BEvmProto";

// CallRequest is a request to execute a view method (read-only).
type CallRequest struct {
	ByzCoinID       []byte
	ServerConfig    string
	BEvmInstanceID  []byte
	AccountAddress  []byte
	ContractAddress []byte
	CallData        []byte
}

// CallResponse is the response to CallRequest, containing the method response.
type CallResponse struct {
	Result []byte
}

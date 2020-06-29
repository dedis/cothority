package bevm

// PROTOSTART
// package bevm;
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "BEvmProto";

// DeployRequest is the request to prepare an EVM transaction to deploy a
// contract.
type DeployRequest struct {
	GasLimit uint64
	GasPrice uint64
	Amount   uint64
	Nonce    uint64
	Bytecode []byte
	// JSON-encoded
	Abi string
	// JSON-encoded
	Args []string
}

// TransactionRequest is the request to prepare an EVM transaction for a R/W
// method execution.
type TransactionRequest struct {
	GasLimit        uint64
	GasPrice        uint64
	Amount          uint64
	ContractAddress []byte
	Nonce           uint64
	// JSON-encoded
	Abi    string
	Method string
	// JSON-encoded
	Args []string
}

// TransactionHashResponse is the response to both DeployRequest and
// TransactionRequest, containing the transaction and its hash to sign.
type TransactionHashResponse struct {
	Transaction     []byte
	TransactionHash []byte
}

// TransactionFinalizationRequest is the request to finalize a transaction with
// its signature.
type TransactionFinalizationRequest struct {
	Transaction []byte
	Signature   []byte
}

// TransactionResponse is the response to TransactionFinalizationRequest,
// containing the signed transaction.
type TransactionResponse struct {
	Transaction []byte
}

// CallRequest is a request to execute a view method (read-only).
type CallRequest struct {
	ByzCoinID       []byte
	ServerConfig    string
	BEvmInstanceID  []byte
	AccountAddress  []byte
	ContractAddress []byte
	// JSON-encoded
	Abi    string
	Method string
	// JSON-encoded
	Args []string
}

// CallResponse is the response to CallRequest, containing the method response.
type CallResponse struct {
	// JSON-encoded
	Result string
}

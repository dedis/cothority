package did

// Sovrin represents the arguments required to resolve a did from the
// Sovrin chain.
type Sovrin struct {
	Pool SovrinPool
}

// SovrinPool represents the information required to connect to a
// sovrin chain.
type SovrinPool struct {
	Name       string
	GenesisTxn string
}

// SovrinDIDProps represents all the arguments required to add/update a
// Sovrin DID in byzcoin.
type SovrinDIDProps struct {
	DID         string
	Transaction GetNymTransaction
	// TODO: Add pool related proofs
}

type GetNymTransaction struct {
	Op     string
	Result GetNymResult
}

type GetNymResult struct {
	Type       string
	Identifier string
	ReqId      string
	SeqNo      string
	TxnTime    string
	StateProof StateProof
	Data       string
	Dest       string
}

type StateProof struct {
	RootHash       string
	ProofNodes     string
	MultiSignature MultiSignature
}

type MultiSignature struct {
	Value        MultiSignatureValue
	Signature    string
	Participants []string
}

type MultiSignatureValue struct {
	Timestamp         string
	LedgerID          int
	TxnRootHash       string
	PoolStateRootHash string
	StateRootHash     string
}

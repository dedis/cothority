package did

type getNymData struct {
	Dest           string `json:"dest"`
	Identifier     string `json:"identifier"`
	Role           string `json:"role,omitempty"`
	SequenceNumber string `json:"seqNo"`
	TxnTime        int64  `json:"txnTime"`
	Verkey         string `json:"verkey"`
}

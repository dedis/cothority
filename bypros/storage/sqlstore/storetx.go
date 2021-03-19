package sqlstore

import (
	"database/sql"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// storeTx handles the storing of a block based on an SQL transaction.
type storeTx struct {
	sqlTx *sql.Tx
	block *skipchain.SkipBlock
}

// store stores the block using the transaction
func (s storeTx) store() (int, error) {
	var blockID int

	query := `INSERT INTO cothority.block (hash) 
	VALUES ($1) 
	RETURNING block_id`

	err := s.sqlTx.QueryRow(query, s.block.Hash).Scan(&blockID)
	if err != nil {
		return -1, xerrors.Errorf("failed to insert block: %v", err)
	}

	var body byzcoin.DataBody

	err = protobuf.Decode(s.block.Payload, &body)
	if err != nil {
		return -1, xerrors.Errorf("failed to decode block payload: %v", err)
	}

	for _, tx := range body.TxResults {
		_, err := s.storeTransaction(tx, blockID)
		if err != nil {
			return -1, xerrors.Errorf("failed to store transaction: %v", err)
		}
	}

	return blockID, nil
}

func (s storeTx) storeTransaction(transaction byzcoin.TxResult, blockID int) (int, error) {
	var transactionID int

	query := `INSERT INTO cothority.transaction (accepted, block_id) 
	VALUES ($1, $2) 
	RETURNING transaction_id`

	err := s.sqlTx.QueryRow(query, transaction.Accepted, blockID).Scan(&transactionID)
	if err != nil {
		return -1, xerrors.Errorf("failed to insert transaction: %v", err)
	}

	for _, instruction := range transaction.ClientTransaction.Instructions {
		_, err := s.storeInstruction(instruction, transactionID)
		if err != nil {
			return -1, xerrors.Errorf("failed to store instruction: %v", err)
		}
	}

	return transactionID, nil
}

func (s storeTx) storeInstruction(instruction byzcoin.Instruction, transactionID int) (int, error) {
	var instructionID int

	instanceIID := instruction.InstanceID
	contractIID := instruction.InstanceID

	if instruction.GetType() == byzcoin.SpawnType {
		contractIID = instruction.DeriveID("")
	}

	if byzcoin.ConfigInstanceID.Equal(instruction.InstanceID) {
		contractIID = byzcoin.ConfigInstanceID
	} else if byzcoin.NamingInstanceID.Equal(instruction.InstanceID) {
		contractIID = byzcoin.NamingInstanceID
	}

	typeID := 1

	switch instruction.GetType() {
	case byzcoin.SpawnType:
		typeID = 2
	case byzcoin.InvokeType:
		typeID = 3
	case byzcoin.DeleteType:
		typeID = 4
	}

	query := `INSERT INTO cothority.instruction (transaction_id, type_id, 
		action, instance_iid, contract_iid, contract_name) 
	VALUES ($1, $2, $3, $4, $5, $6) 
	RETURNING instruction_id`

	err := s.sqlTx.QueryRow(query, transactionID, typeID, instruction.Action(),
		instanceIID.Slice(), contractIID.Slice(),
		instruction.ContractID()).Scan(&instructionID)
	if err != nil {
		return -1, xerrors.Errorf("failed to insert transaction: %v", err)
	}

	if len(instruction.SignerIdentities) != len(instruction.SignerCounter) &&
		len(instruction.SignerIdentities) != len(instruction.SignerCounter) {

		return -1, xerrors.Errorf("invalid instruction: %v", instruction)
	}

	for i := 0; i < len(instruction.Signatures); i++ {
		identity := instruction.SignerIdentities[i].String()
		signature := instruction.Signatures[i]
		counter := instruction.SignerCounter[i]

		_, err := s.storeSigner(identity, signature, counter, instructionID)
		if err != nil {
			return -1, xerrors.Errorf("failed to store signer: %v", err)
		}
	}

	for _, name := range instruction.Arguments().Names() {
		value := instruction.Arguments().Search(name)

		_, err := s.storeArgument(name, value, instructionID)
		if err != nil {
			return -1, xerrors.Errorf("failed to store argument: %v", err)
		}
	}

	return instructionID, nil
}

func (s storeTx) storeSigner(identity string, signature []byte, counter uint64,
	instructionID int) (int, error) {

	var signerID int

	query := `INSERT INTO cothority.signer (identity, signature, counter, 
		instruction_id) 
	VALUES ($1, $2, $3, $4) 
	RETURNING signer_id`

	err := s.sqlTx.QueryRow(query, identity, signature, counter,
		instructionID).Scan(&signerID)
	if err != nil {
		return -1, xerrors.Errorf("failed to insert signer: %v", err)
	}

	return signerID, nil
}

func (s storeTx) storeArgument(name string, value []byte, instructionID int) (int, error) {
	var argumentID int

	query := `INSERT INTO cothority.argument (name, value, instruction_id) 
	VALUES ($1, $2, $3) 
	RETURNING argument_id`

	err := s.sqlTx.QueryRow(query, name, value, instructionID).Scan(&argumentID)
	if err != nil {
		return -1, xerrors.Errorf("failed to insert signer: %v", err)
	}

	return argumentID, nil
}

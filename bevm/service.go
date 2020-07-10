package bevm

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
)

// ServiceName is the name to refer to the BEvm service.
const ServiceName = "BEvm"

// Service is the service that performs BEvm operations.
type Service struct {
	*onet.ServiceProcessor
}

func init() {
	// Ethereum starts goroutines for caching transactions, and never
	// terminates them
	log.AddUserUninterestingGoroutine(
		"go-ethereum/core.(*txSenderCacher).cache")

	// Register contracts
	log.ErrFatal(byzcoin.RegisterGlobalContract(ContractBEvmID,
		contractBEvmFromBytes))
	log.ErrFatal(byzcoin.RegisterGlobalContract(ContractBEvmValueID,
		nil))

	// Initialize service
	_, err := onet.RegisterNewService(ServiceName, newBEvmService)
	log.ErrFatal(err)
}

// PrepareDeployTx builds a transaction to deploy an EVM contract. Returns an
// EVM transaction and its hash to be signed by the caller.
func (service *Service) PrepareDeployTx(
	req *DeployRequest) (*TransactionHashResponse, error) {
	abi, err := abi.JSON(strings.NewReader(req.Abi))
	if err != nil {
		return nil, xerrors.Errorf("failed to decode JSON ABI: %v", err)
	}

	args, err := DecodeEvmArgs(req.Args, abi.Constructor.Inputs)
	if err != nil {
		return nil, xerrors.Errorf("failed to decode tx args: %v", err)
	}

	packedArgs, err := abi.Pack("", args...)
	if err != nil {
		return nil, xerrors.Errorf("failed to pack tx args: %v", err)
	}

	callData := append(req.Bytecode, packedArgs...)

	tx := types.NewContractCreation(req.Nonce, big.NewInt(int64(req.Amount)),
		req.GasLimit, big.NewInt(int64(req.GasPrice)), callData)

	signer := types.HomesteadSigner{}
	hashedTx := signer.Hash(tx)

	unsignedBuffer, err := tx.MarshalJSON()
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal tx to JSON: %v", err)
	}

	log.Lvl4("Returning", unsignedBuffer, hashedTx)

	return &TransactionHashResponse{Transaction: unsignedBuffer,
		TransactionHash: hashedTx[:]}, nil
}

// PrepareTransactionTx builds a transaction to execute a R/W method on a
// previously deployed EVM contract instance. Returns an EVM transaction and
// its hash to be signed by the caller.
func (service *Service) PrepareTransactionTx(
	req *TransactionRequest) (*TransactionHashResponse, error) {
	abi, err := abi.JSON(strings.NewReader(req.Abi))
	if err != nil {
		return nil, xerrors.Errorf("failed to decode JSON ABI: %v", err)
	}

	args, err := DecodeEvmArgs(req.Args, abi.Methods[req.Method].Inputs)
	if err != nil {
		return nil, xerrors.Errorf("failed to decode tx args: %v", err)
	}

	callData, err := abi.Pack(req.Method, args...)
	if err != nil {
		return nil, xerrors.Errorf("failed to pack args: %v", err)
	}

	tx := types.NewTransaction(req.Nonce,
		common.BytesToAddress(req.ContractAddress),
		big.NewInt(int64(req.Amount)),
		req.GasLimit, big.NewInt(int64(req.GasPrice)), callData)

	signer := types.HomesteadSigner{}
	hashedTx := signer.Hash(tx)

	unsignedBuffer, err := tx.MarshalJSON()
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal tx to JSON: %v", err)
	}

	log.Lvl4("Returning", unsignedBuffer, hashedTx)

	return &TransactionHashResponse{Transaction: unsignedBuffer,
		TransactionHash: hashedTx[:]}, nil
}

// FinalizeTx finalizes a previously initiated transaction, signed by the
// caller. Returns an EVM transaction ready to be sent to ByzCoin and handled
// by the bevm contract.
func (service *Service) FinalizeTx(
	req *TransactionFinalizationRequest) (*TransactionResponse, error) {
	signer := types.HomesteadSigner{}

	var tx types.Transaction
	err := tx.UnmarshalJSON(req.Transaction)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal tx from JSON: %v", err)
	}

	signedTx, err := tx.WithSignature(signer, req.Signature)
	if err != nil {
		return nil, xerrors.Errorf("failed to sign tx: %v", err)
	}

	signedBuffer, err := signedTx.MarshalJSON()
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal signed tx"+
			"to JSON: %v", err)
	}

	log.Lvl4("Returning", signedBuffer)

	return &TransactionResponse{
		Transaction: signedBuffer,
	}, nil
}

// PerformCall executes a R-only method on a previously deployed EVM contract
// instance by contacting a ByzCoin cothority. Returns the call response.
func (service *Service) PerformCall(req *CallRequest) (*CallResponse,
	error) {
	abi, err := abi.JSON(strings.NewReader(req.Abi))
	if err != nil {
		return nil, xerrors.Errorf("failed to decode JSON ABI: %v", err)
	}

	methodAbi, ok := abi.Methods[req.Method]
	if !ok {
		return nil, xerrors.Errorf("method '%s' does not exist", req.Method)
	}

	args, err := DecodeEvmArgs(req.Args, methodAbi.Inputs)
	if err != nil {
		return nil, xerrors.Errorf("failed to decode view method args: %v", err)
	}

	// We don't need the private key for reading proofs
	account := &EvmAccount{
		Address: common.BytesToAddress(req.AccountAddress),
	}
	// We don't need the bytecode
	contractInstance := EvmContractInstance{
		Parent: &EvmContract{
			Abi: abi,
		},
		Address: common.BytesToAddress(req.ContractAddress),
	}

	// Read server configuration from TOML data
	grp, err := app.ReadGroupDescToml(strings.NewReader(req.ServerConfig))
	if err != nil {
		return nil, xerrors.Errorf("failed to read server TOML config: %v", err)
	}
	// Instantiate a new ByzCoin client
	bcClient := byzcoin.NewClient(req.ByzCoinID, *grp.Roster)

	// Instantiate a new BEvm client (we don't need a darc to read proofs)
	bevmClient, err := NewClient(bcClient, darc.Signer{},
		byzcoin.NewInstanceID(req.BEvmInstanceID))
	if err != nil {
		return nil, xerrors.Errorf("failed to create new BEvm client: %v", err)
	}

	// Execute the view method in the EVM
	result, err := bevmClient.Call(account, &contractInstance,
		req.Method, args...)
	if err != nil {
		return nil, xerrors.Errorf("failed to perform BEvm call: %v", err)
	}

	log.Lvlf4("Returning: %v", result)

	resultJSON, err := EncodeEvmReturnValue(result, methodAbi.Outputs)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal result to JSON: %v", err)
	}

	return &CallResponse{Result: resultJSON}, nil
}

// newBEvmService creates a new service for BEvm functionality
func newBEvmService(context *onet.Context) (onet.Service, error) {
	service := &Service{
		ServiceProcessor: onet.NewServiceProcessor(context),
	}

	err := service.RegisterHandlers(
		service.PrepareDeployTx,
		service.PrepareTransactionTx,
		service.FinalizeTx,
		service.PerformCall,
	)
	if err != nil {
		return nil, xerrors.Errorf("failed to register service "+
			"handlers: %v", err)
	}

	return service, nil
}

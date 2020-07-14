package bevm

import (
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/common"
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

// PerformCall executes a R-only method on a previously deployed EVM contract
// instance by contacting a ByzCoin cothority. Returns the call response.
func (service *Service) PerformCall(req *CallRequest) (*CallResponse,
	error) {
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

	accountAddress := common.BytesToAddress(req.AccountAddress)
	contractAddress := common.BytesToAddress(req.ContractAddress)

	// Execute the view method in the EVM
	result, err := bevmClient.CallPacked(accountAddress, contractAddress,
		req.CallData)
	if err != nil {
		return nil, xerrors.Errorf("failed to perform BEvm call: %v", err)
	}

	log.Lvlf4("Returning: %v", hex.EncodeToString(result))

	return &CallResponse{Result: result}, nil
}

// newBEvmService creates a new service for BEvm functionality
func newBEvmService(context *onet.Context) (onet.Service, error) {
	service := &Service{
		ServiceProcessor: onet.NewServiceProcessor(context),
	}

	err := service.RegisterHandlers(
		service.PerformCall,
	)
	if err != nil {
		return nil, xerrors.Errorf("failed to register service "+
			"handlers: %v", err)
	}

	return service, nil
}

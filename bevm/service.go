package bevm

import (
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3"
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

// Retrieve the StateTrie, waiting some time in case the minimum block index
// required has not yet been reached.
func getReadOnlyStateTrie(bcService *byzcoin.Service, byzcoinID []byte,
	minBlockIndex int) (rst byzcoin.ReadOnlyStateTrie, err error) {
	const maxTries = 3

	for triesLeft := maxTries; ; {
		rst, err = bcService.GetReadOnlyStateTrie(byzcoinID)
		if err != nil {
			return nil, xerrors.Errorf("failed to retrieve ReadOnlyStateTrie: "+
				"%v", err)
		}

		currentIndex := rst.GetIndex()

		log.Lvlf2("[%v] rst index: %v >= %v?", maxTries-triesLeft,
			currentIndex, minBlockIndex)

		if currentIndex >= minBlockIndex {
			break
		}

		triesLeft--
		if triesLeft == 0 {
			// Give up
			return nil, xerrors.Errorf("failed to reach minimum block "+
				"(%v < %v)", rst.GetIndex(), minBlockIndex)
		}

		time.Sleep(1 * time.Second)
	}

	return
}

// ViewCall executes a R-only method on a previously deployed EVM contract
// instance by contacting a ByzCoin cothority. Returns the call response.
func (service *Service) ViewCall(req *ViewCallRequest) (*ViewCallResponse,
	error) {
	accountAddress := common.BytesToAddress(req.AccountAddress)
	contractAddress := common.BytesToAddress(req.ContractAddress)

	serv := service.Context.Service(byzcoin.ServiceName)
	if serv == nil {
		return nil, xerrors.New("cannot find \"byzcoin\" service")
	}

	bcService, ok := serv.(*byzcoin.Service)
	if !ok {
		return nil,
			xerrors.New("internal error: service is not a byzcoin.Service")
	}

	rst, err := getReadOnlyStateTrie(bcService, req.ByzCoinID, req.MinBlockIndex)
	if err != nil {
		return nil, xerrors.Errorf("failed to retrieve ReadOnlyStateTrie: %v",
			err)
	}

	// Retrieve the EVM state
	stateDb, err := getEvmDbRst(rst, byzcoin.NewInstanceID(req.BEvmInstanceID))
	if err != nil {
		return nil, xerrors.Errorf("failed to obtain stateTrie-backed database "+
			"for BEvm: %v", err)
	}

	// Compute timestamp for the EVM
	timestamp := time.Now().UnixNano()
	// timestamp in ByzCoin is in [ns], whereas in EVM it is in [s]
	evmTs := timestamp / 1e9

	result, err := CallEVM(accountAddress, contractAddress, req.CallData,
		stateDb, evmTs)
	if err != nil {
		return nil, xerrors.Errorf("failed to execute EVM view "+
			"method: %v", err)
	}

	log.Lvlf4("Returning: %v", hex.EncodeToString(result))

	return &ViewCallResponse{Result: result}, nil
}

// newBEvmService creates a new service for BEvm functionality
func newBEvmService(context *onet.Context) (onet.Service, error) {
	service := &Service{
		ServiceProcessor: onet.NewServiceProcessor(context),
	}

	err := service.RegisterHandlers(
		service.ViewCall,
	)
	if err != nil {
		return nil, xerrors.Errorf("failed to register service "+
			"handlers: %v", err)
	}

	return service, nil
}

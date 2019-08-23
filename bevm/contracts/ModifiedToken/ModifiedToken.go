// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package store

import (
	"math/big"
	"strings"

	ethereum "github.com/c4dt/go-ethereum"
	"github.com/c4dt/go-ethereum/accounts/abi"
	"github.com/c4dt/go-ethereum/accounts/abi/bind"
	"github.com/c4dt/go-ethereum/common"
	"github.com/c4dt/go-ethereum/core/types"
	"github.com/c4dt/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = abi.U256
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// StoreABI is the input ABI used to generate the binding from.
const StoreABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_value\",\"type\":\"uint64\"}],\"name\":\"transfer\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"name\":\"\",\"type\":\"uint64\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"initialSupply\",\"type\":\"uint64\"},{\"name\":\"toGiveTo\",\"type\":\"address\"}],\"name\":\"create\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint64\"}],\"name\":\"send\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"name\":\"getBalance\",\"outputs\":[{\"name\":\"\",\"type\":\"uint64\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// Store is an auto generated Go binding around an Ethereum contract.
type Store struct {
	StoreCaller     // Read-only binding to the contract
	StoreTransactor // Write-only binding to the contract
	StoreFilterer   // Log filterer for contract events
}

// StoreCaller is an auto generated read-only Go binding around an Ethereum contract.
type StoreCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StoreTransactor is an auto generated write-only Go binding around an Ethereum contract.
type StoreTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StoreFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type StoreFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StoreSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type StoreSession struct {
	Contract     *Store            // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// StoreCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type StoreCallerSession struct {
	Contract *StoreCaller  // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// StoreTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type StoreTransactorSession struct {
	Contract     *StoreTransactor  // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// StoreRaw is an auto generated low-level Go binding around an Ethereum contract.
type StoreRaw struct {
	Contract *Store // Generic contract binding to access the raw methods on
}

// StoreCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type StoreCallerRaw struct {
	Contract *StoreCaller // Generic read-only contract binding to access the raw methods on
}

// StoreTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type StoreTransactorRaw struct {
	Contract *StoreTransactor // Generic write-only contract binding to access the raw methods on
}

// NewStore creates a new instance of Store, bound to a specific deployed contract.
func NewStore(address common.Address, backend bind.ContractBackend) (*Store, error) {
	contract, err := bindStore(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Store{StoreCaller: StoreCaller{contract: contract}, StoreTransactor: StoreTransactor{contract: contract}, StoreFilterer: StoreFilterer{contract: contract}}, nil
}

// NewStoreCaller creates a new read-only instance of Store, bound to a specific deployed contract.
func NewStoreCaller(address common.Address, caller bind.ContractCaller) (*StoreCaller, error) {
	contract, err := bindStore(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &StoreCaller{contract: contract}, nil
}

// NewStoreTransactor creates a new write-only instance of Store, bound to a specific deployed contract.
func NewStoreTransactor(address common.Address, transactor bind.ContractTransactor) (*StoreTransactor, error) {
	contract, err := bindStore(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &StoreTransactor{contract: contract}, nil
}

// NewStoreFilterer creates a new log filterer instance of Store, bound to a specific deployed contract.
func NewStoreFilterer(address common.Address, filterer bind.ContractFilterer) (*StoreFilterer, error) {
	contract, err := bindStore(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &StoreFilterer{contract: contract}, nil
}

// bindStore binds a generic wrapper to an already deployed contract.
func bindStore(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(StoreABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Store *StoreRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Store.Contract.StoreCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Store *StoreRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Store.Contract.StoreTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Store *StoreRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Store.Contract.StoreTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Store *StoreCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Store.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Store *StoreTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Store.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Store *StoreTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Store.Contract.contract.Transact(opts, method, params...)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address ) constant returns(uint64)
func (_Store *StoreCaller) BalanceOf(opts *bind.CallOpts, arg0 common.Address) (uint64, error) {
	var (
		ret0 = new(uint64)
	)
	out := ret0
	err := _Store.contract.Call(opts, out, "balanceOf", arg0)
	return *ret0, err
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address ) constant returns(uint64)
func (_Store *StoreSession) BalanceOf(arg0 common.Address) (uint64, error) {
	return _Store.Contract.BalanceOf(&_Store.CallOpts, arg0)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address ) constant returns(uint64)
func (_Store *StoreCallerSession) BalanceOf(arg0 common.Address) (uint64, error) {
	return _Store.Contract.BalanceOf(&_Store.CallOpts, arg0)
}

// GetBalance is a free data retrieval call binding the contract method 0xf8b2cb4f.
//
// Solidity: function getBalance(address account) constant returns(uint64)
func (_Store *StoreCaller) GetBalance(opts *bind.CallOpts, account common.Address) (uint64, error) {
	var (
		ret0 = new(uint64)
	)
	out := ret0
	err := _Store.contract.Call(opts, out, "getBalance", account)
	return *ret0, err
}

// GetBalance is a free data retrieval call binding the contract method 0xf8b2cb4f.
//
// Solidity: function getBalance(address account) constant returns(uint64)
func (_Store *StoreSession) GetBalance(account common.Address) (uint64, error) {
	return _Store.Contract.GetBalance(&_Store.CallOpts, account)
}

// GetBalance is a free data retrieval call binding the contract method 0xf8b2cb4f.
//
// Solidity: function getBalance(address account) constant returns(uint64)
func (_Store *StoreCallerSession) GetBalance(account common.Address) (uint64, error) {
	return _Store.Contract.GetBalance(&_Store.CallOpts, account)
}

// Create is a paid mutator transaction binding the contract method 0x8d3881b1.
//
// Solidity: function create(uint64 initialSupply, address toGiveTo) returns()
func (_Store *StoreTransactor) Create(opts *bind.TransactOpts, initialSupply uint64, toGiveTo common.Address) (*types.Transaction, error) {
	return _Store.contract.Transact(opts, "create", initialSupply, toGiveTo)
}

// Create is a paid mutator transaction binding the contract method 0x8d3881b1.
//
// Solidity: function create(uint64 initialSupply, address toGiveTo) returns()
func (_Store *StoreSession) Create(initialSupply uint64, toGiveTo common.Address) (*types.Transaction, error) {
	return _Store.Contract.Create(&_Store.TransactOpts, initialSupply, toGiveTo)
}

// Create is a paid mutator transaction binding the contract method 0x8d3881b1.
//
// Solidity: function create(uint64 initialSupply, address toGiveTo) returns()
func (_Store *StoreTransactorSession) Create(initialSupply uint64, toGiveTo common.Address) (*types.Transaction, error) {
	return _Store.Contract.Create(&_Store.TransactOpts, initialSupply, toGiveTo)
}

// Send is a paid mutator transaction binding the contract method 0xf20d643b.
//
// Solidity: function send(address _from, address _to, uint64 _amount) returns(bool)
func (_Store *StoreTransactor) Send(opts *bind.TransactOpts, _from common.Address, _to common.Address, _amount uint64) (*types.Transaction, error) {
	return _Store.contract.Transact(opts, "send", _from, _to, _amount)
}

// Send is a paid mutator transaction binding the contract method 0xf20d643b.
//
// Solidity: function send(address _from, address _to, uint64 _amount) returns(bool)
func (_Store *StoreSession) Send(_from common.Address, _to common.Address, _amount uint64) (*types.Transaction, error) {
	return _Store.Contract.Send(&_Store.TransactOpts, _from, _to, _amount)
}

// Send is a paid mutator transaction binding the contract method 0xf20d643b.
//
// Solidity: function send(address _from, address _to, uint64 _amount) returns(bool)
func (_Store *StoreTransactorSession) Send(_from common.Address, _to common.Address, _amount uint64) (*types.Transaction, error) {
	return _Store.Contract.Send(&_Store.TransactOpts, _from, _to, _amount)
}

// Transfer is a paid mutator transaction binding the contract method 0x2a308b3a.
//
// Solidity: function transfer(address _from, address _to, uint64 _value) returns(bool)
func (_Store *StoreTransactor) Transfer(opts *bind.TransactOpts, _from common.Address, _to common.Address, _value uint64) (*types.Transaction, error) {
	return _Store.contract.Transact(opts, "transfer", _from, _to, _value)
}

// Transfer is a paid mutator transaction binding the contract method 0x2a308b3a.
//
// Solidity: function transfer(address _from, address _to, uint64 _value) returns(bool)
func (_Store *StoreSession) Transfer(_from common.Address, _to common.Address, _value uint64) (*types.Transaction, error) {
	return _Store.Contract.Transfer(&_Store.TransactOpts, _from, _to, _value)
}

// Transfer is a paid mutator transaction binding the contract method 0x2a308b3a.
//
// Solidity: function transfer(address _from, address _to, uint64 _value) returns(bool)
func (_Store *StoreTransactorSession) Transfer(_from common.Address, _to common.Address, _value uint64) (*types.Transaction, error) {
	return _Store.Contract.Transfer(&_Store.TransactOpts, _from, _to, _value)
}

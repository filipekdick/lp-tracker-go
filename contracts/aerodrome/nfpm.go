// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package aerodrome

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// INonfungiblePositionManagerCollectParams is an auto generated low-level Go binding around an user-defined struct.
type INonfungiblePositionManagerCollectParams struct {
	TokenId    *big.Int
	Recipient  common.Address
	Amount0Max *big.Int
	Amount1Max *big.Int
}

// INonfungiblePositionManagerDecreaseLiquidityParams is an auto generated low-level Go binding around an user-defined struct.
type INonfungiblePositionManagerDecreaseLiquidityParams struct {
	TokenId    *big.Int
	Liquidity  *big.Int
	Amount0Min *big.Int
	Amount1Min *big.Int
	Deadline   *big.Int
}

// INonfungiblePositionManagerIncreaseLiquidityParams is an auto generated low-level Go binding around an user-defined struct.
type INonfungiblePositionManagerIncreaseLiquidityParams struct {
	TokenId        *big.Int
	Amount0Desired *big.Int
	Amount1Desired *big.Int
	Amount0Min     *big.Int
	Amount1Min     *big.Int
	Deadline       *big.Int
}

// INonfungiblePositionManagerMintParams is an auto generated low-level Go binding around an user-defined struct.
type INonfungiblePositionManagerMintParams struct {
	Token0         common.Address
	Token1         common.Address
	TickSpacing    *big.Int
	TickLower      *big.Int
	TickUpper      *big.Int
	Amount0Desired *big.Int
	Amount1Desired *big.Int
	Amount0Min     *big.Int
	Amount1Min     *big.Int
	Recipient      common.Address
	Deadline       *big.Int
	SqrtPriceX96   *big.Int
}

// NFPMMetaData contains all meta data concerning the NFPM contract.
var NFPMMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_factory\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_WETH9\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_tokenDescriptor\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"symbol\",\"type\":\"string\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"approved\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"ApprovalForAll\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"_fromTokenId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"_toTokenId\",\"type\":\"uint256\"}],\"name\":\"BatchMetadataUpdate\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount0\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount1\",\"type\":\"uint256\"}],\"name\":\"Collect\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint128\",\"name\":\"liquidity\",\"type\":\"uint128\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount0\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount1\",\"type\":\"uint256\"}],\"name\":\"DecreaseLiquidity\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint128\",\"name\":\"liquidity\",\"type\":\"uint128\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount0\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount1\",\"type\":\"uint256\"}],\"name\":\"IncreaseLiquidity\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"_tokenId\",\"type\":\"uint256\"}],\"name\":\"MetadataUpdate\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"tokenDescriptor\",\"type\":\"address\"}],\"name\":\"TokenDescriptorChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"TransferOwnership\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DOMAIN_SEPARATOR\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"PERMIT_TYPEHASH\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WETH9\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"baseURI\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"burn\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"internalType\":\"uint128\",\"name\":\"amount0Max\",\"type\":\"uint128\"},{\"internalType\":\"uint128\",\"name\":\"amount1Max\",\"type\":\"uint128\"}],\"internalType\":\"structINonfungiblePositionManager.CollectParams\",\"name\":\"params\",\"type\":\"tuple\"}],\"name\":\"collect\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"amount0\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"uint128\",\"name\":\"liquidity\",\"type\":\"uint128\"},{\"internalType\":\"uint256\",\"name\":\"amount0Min\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1Min\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"deadline\",\"type\":\"uint256\"}],\"internalType\":\"structINonfungiblePositionManager.DecreaseLiquidityParams\",\"name\":\"params\",\"type\":\"tuple\"}],\"name\":\"decreaseLiquidity\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"amount0\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"factory\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"getApproved\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount0Desired\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1Desired\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount0Min\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1Min\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"deadline\",\"type\":\"uint256\"}],\"internalType\":\"structINonfungiblePositionManager.IncreaseLiquidityParams\",\"name\":\"params\",\"type\":\"tuple\"}],\"name\":\"increaseLiquidity\",\"outputs\":[{\"internalType\":\"uint128\",\"name\":\"liquidity\",\"type\":\"uint128\"},{\"internalType\":\"uint256\",\"name\":\"amount0\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isApprovedForAll\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"internalType\":\"int24\",\"name\":\"tickSpacing\",\"type\":\"int24\"},{\"internalType\":\"int24\",\"name\":\"tickLower\",\"type\":\"int24\"},{\"internalType\":\"int24\",\"name\":\"tickUpper\",\"type\":\"int24\"},{\"internalType\":\"uint256\",\"name\":\"amount0Desired\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1Desired\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount0Min\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1Min\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"deadline\",\"type\":\"uint256\"},{\"internalType\":\"uint160\",\"name\":\"sqrtPriceX96\",\"type\":\"uint160\"}],\"internalType\":\"structINonfungiblePositionManager.MintParams\",\"name\":\"params\",\"type\":\"tuple\"}],\"name\":\"mint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"uint128\",\"name\":\"liquidity\",\"type\":\"uint128\"},{\"internalType\":\"uint256\",\"name\":\"amount0\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes[]\",\"name\":\"data\",\"type\":\"bytes[]\"}],\"name\":\"multicall\",\"outputs\":[{\"internalType\":\"bytes[]\",\"name\":\"results\",\"type\":\"bytes[]\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"ownerOf\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"deadline\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"v\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"r\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"s\",\"type\":\"bytes32\"}],\"name\":\"permit\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"positions\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"nonce\",\"type\":\"uint96\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"internalType\":\"int24\",\"name\":\"tickSpacing\",\"type\":\"int24\"},{\"internalType\":\"int24\",\"name\":\"tickLower\",\"type\":\"int24\"},{\"internalType\":\"int24\",\"name\":\"tickUpper\",\"type\":\"int24\"},{\"internalType\":\"uint128\",\"name\":\"liquidity\",\"type\":\"uint128\"},{\"internalType\":\"uint256\",\"name\":\"feeGrowthInside0LastX128\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"feeGrowthInside1LastX128\",\"type\":\"uint256\"},{\"internalType\":\"uint128\",\"name\":\"tokensOwed0\",\"type\":\"uint128\"},{\"internalType\":\"uint128\",\"name\":\"tokensOwed1\",\"type\":\"uint128\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"refundETH\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"deadline\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"v\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"r\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"s\",\"type\":\"bytes32\"}],\"name\":\"selfPermit\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"expiry\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"v\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"r\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"s\",\"type\":\"bytes32\"}],\"name\":\"selfPermitAllowed\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"expiry\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"v\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"r\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"s\",\"type\":\"bytes32\"}],\"name\":\"selfPermitAllowedIfNecessary\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"deadline\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"v\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"r\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"s\",\"type\":\"bytes32\"}],\"name\":\"selfPermitIfNecessary\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"setApprovalForAll\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"setOwner\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_tokenDescriptor\",\"type\":\"address\"}],\"name\":\"setTokenDescriptor\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes4\",\"name\":\"interfaceId\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amountMinimum\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"}],\"name\":\"sweepToken\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"tokenByIndex\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tokenDescriptor\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"tokenOfOwnerByIndex\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"tokenURI\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount0Owed\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount1Owed\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"uniswapV3MintCallback\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amountMinimum\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"}],\"name\":\"unwrapWETH9\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
}

// NFPMABI is the input ABI used to generate the binding from.
// Deprecated: Use NFPMMetaData.ABI instead.
var NFPMABI = NFPMMetaData.ABI

// NFPM is an auto generated Go binding around an Ethereum contract.
type NFPM struct {
	NFPMCaller     // Read-only binding to the contract
	NFPMTransactor // Write-only binding to the contract
	NFPMFilterer   // Log filterer for contract events
}

// NFPMCaller is an auto generated read-only Go binding around an Ethereum contract.
type NFPMCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NFPMTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NFPMTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NFPMFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NFPMFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NFPMSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NFPMSession struct {
	Contract     *NFPM             // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NFPMCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NFPMCallerSession struct {
	Contract *NFPMCaller   // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// NFPMTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NFPMTransactorSession struct {
	Contract     *NFPMTransactor   // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NFPMRaw is an auto generated low-level Go binding around an Ethereum contract.
type NFPMRaw struct {
	Contract *NFPM // Generic contract binding to access the raw methods on
}

// NFPMCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NFPMCallerRaw struct {
	Contract *NFPMCaller // Generic read-only contract binding to access the raw methods on
}

// NFPMTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NFPMTransactorRaw struct {
	Contract *NFPMTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNFPM creates a new instance of NFPM, bound to a specific deployed contract.
func NewNFPM(address common.Address, backend bind.ContractBackend) (*NFPM, error) {
	contract, err := bindNFPM(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &NFPM{NFPMCaller: NFPMCaller{contract: contract}, NFPMTransactor: NFPMTransactor{contract: contract}, NFPMFilterer: NFPMFilterer{contract: contract}}, nil
}

// NewNFPMCaller creates a new read-only instance of NFPM, bound to a specific deployed contract.
func NewNFPMCaller(address common.Address, caller bind.ContractCaller) (*NFPMCaller, error) {
	contract, err := bindNFPM(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NFPMCaller{contract: contract}, nil
}

// NewNFPMTransactor creates a new write-only instance of NFPM, bound to a specific deployed contract.
func NewNFPMTransactor(address common.Address, transactor bind.ContractTransactor) (*NFPMTransactor, error) {
	contract, err := bindNFPM(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NFPMTransactor{contract: contract}, nil
}

// NewNFPMFilterer creates a new log filterer instance of NFPM, bound to a specific deployed contract.
func NewNFPMFilterer(address common.Address, filterer bind.ContractFilterer) (*NFPMFilterer, error) {
	contract, err := bindNFPM(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NFPMFilterer{contract: contract}, nil
}

// bindNFPM binds a generic wrapper to an already deployed contract.
func bindNFPM(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := NFPMMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NFPM *NFPMRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NFPM.Contract.NFPMCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NFPM *NFPMRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NFPM.Contract.NFPMTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NFPM *NFPMRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NFPM.Contract.NFPMTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NFPM *NFPMCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NFPM.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NFPM *NFPMTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NFPM.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NFPM *NFPMTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NFPM.Contract.contract.Transact(opts, method, params...)
}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_NFPM *NFPMCaller) DOMAINSEPARATOR(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "DOMAIN_SEPARATOR")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_NFPM *NFPMSession) DOMAINSEPARATOR() ([32]byte, error) {
	return _NFPM.Contract.DOMAINSEPARATOR(&_NFPM.CallOpts)
}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_NFPM *NFPMCallerSession) DOMAINSEPARATOR() ([32]byte, error) {
	return _NFPM.Contract.DOMAINSEPARATOR(&_NFPM.CallOpts)
}

// PERMITTYPEHASH is a free data retrieval call binding the contract method 0x30adf81f.
//
// Solidity: function PERMIT_TYPEHASH() view returns(bytes32)
func (_NFPM *NFPMCaller) PERMITTYPEHASH(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "PERMIT_TYPEHASH")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// PERMITTYPEHASH is a free data retrieval call binding the contract method 0x30adf81f.
//
// Solidity: function PERMIT_TYPEHASH() view returns(bytes32)
func (_NFPM *NFPMSession) PERMITTYPEHASH() ([32]byte, error) {
	return _NFPM.Contract.PERMITTYPEHASH(&_NFPM.CallOpts)
}

// PERMITTYPEHASH is a free data retrieval call binding the contract method 0x30adf81f.
//
// Solidity: function PERMIT_TYPEHASH() view returns(bytes32)
func (_NFPM *NFPMCallerSession) PERMITTYPEHASH() ([32]byte, error) {
	return _NFPM.Contract.PERMITTYPEHASH(&_NFPM.CallOpts)
}

// WETH9 is a free data retrieval call binding the contract method 0x4aa4a4fc.
//
// Solidity: function WETH9() view returns(address)
func (_NFPM *NFPMCaller) WETH9(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "WETH9")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// WETH9 is a free data retrieval call binding the contract method 0x4aa4a4fc.
//
// Solidity: function WETH9() view returns(address)
func (_NFPM *NFPMSession) WETH9() (common.Address, error) {
	return _NFPM.Contract.WETH9(&_NFPM.CallOpts)
}

// WETH9 is a free data retrieval call binding the contract method 0x4aa4a4fc.
//
// Solidity: function WETH9() view returns(address)
func (_NFPM *NFPMCallerSession) WETH9() (common.Address, error) {
	return _NFPM.Contract.WETH9(&_NFPM.CallOpts)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_NFPM *NFPMCaller) BalanceOf(opts *bind.CallOpts, owner common.Address) (*big.Int, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "balanceOf", owner)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_NFPM *NFPMSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _NFPM.Contract.BalanceOf(&_NFPM.CallOpts, owner)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_NFPM *NFPMCallerSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _NFPM.Contract.BalanceOf(&_NFPM.CallOpts, owner)
}

// BaseURI is a free data retrieval call binding the contract method 0x6c0360eb.
//
// Solidity: function baseURI() pure returns(string)
func (_NFPM *NFPMCaller) BaseURI(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "baseURI")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// BaseURI is a free data retrieval call binding the contract method 0x6c0360eb.
//
// Solidity: function baseURI() pure returns(string)
func (_NFPM *NFPMSession) BaseURI() (string, error) {
	return _NFPM.Contract.BaseURI(&_NFPM.CallOpts)
}

// BaseURI is a free data retrieval call binding the contract method 0x6c0360eb.
//
// Solidity: function baseURI() pure returns(string)
func (_NFPM *NFPMCallerSession) BaseURI() (string, error) {
	return _NFPM.Contract.BaseURI(&_NFPM.CallOpts)
}

// Factory is a free data retrieval call binding the contract method 0xc45a0155.
//
// Solidity: function factory() view returns(address)
func (_NFPM *NFPMCaller) Factory(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "factory")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Factory is a free data retrieval call binding the contract method 0xc45a0155.
//
// Solidity: function factory() view returns(address)
func (_NFPM *NFPMSession) Factory() (common.Address, error) {
	return _NFPM.Contract.Factory(&_NFPM.CallOpts)
}

// Factory is a free data retrieval call binding the contract method 0xc45a0155.
//
// Solidity: function factory() view returns(address)
func (_NFPM *NFPMCallerSession) Factory() (common.Address, error) {
	return _NFPM.Contract.Factory(&_NFPM.CallOpts)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_NFPM *NFPMCaller) GetApproved(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "getApproved", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_NFPM *NFPMSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _NFPM.Contract.GetApproved(&_NFPM.CallOpts, tokenId)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_NFPM *NFPMCallerSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _NFPM.Contract.GetApproved(&_NFPM.CallOpts, tokenId)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_NFPM *NFPMCaller) IsApprovedForAll(opts *bind.CallOpts, owner common.Address, operator common.Address) (bool, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "isApprovedForAll", owner, operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_NFPM *NFPMSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _NFPM.Contract.IsApprovedForAll(&_NFPM.CallOpts, owner, operator)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_NFPM *NFPMCallerSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _NFPM.Contract.IsApprovedForAll(&_NFPM.CallOpts, owner, operator)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_NFPM *NFPMCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_NFPM *NFPMSession) Name() (string, error) {
	return _NFPM.Contract.Name(&_NFPM.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_NFPM *NFPMCallerSession) Name() (string, error) {
	return _NFPM.Contract.Name(&_NFPM.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NFPM *NFPMCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NFPM *NFPMSession) Owner() (common.Address, error) {
	return _NFPM.Contract.Owner(&_NFPM.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NFPM *NFPMCallerSession) Owner() (common.Address, error) {
	return _NFPM.Contract.Owner(&_NFPM.CallOpts)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_NFPM *NFPMCaller) OwnerOf(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "ownerOf", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_NFPM *NFPMSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _NFPM.Contract.OwnerOf(&_NFPM.CallOpts, tokenId)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_NFPM *NFPMCallerSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _NFPM.Contract.OwnerOf(&_NFPM.CallOpts, tokenId)
}

// Positions is a free data retrieval call binding the contract method 0x99fbab88.
//
// Solidity: function positions(uint256 tokenId) view returns(uint96 nonce, address operator, address token0, address token1, int24 tickSpacing, int24 tickLower, int24 tickUpper, uint128 liquidity, uint256 feeGrowthInside0LastX128, uint256 feeGrowthInside1LastX128, uint128 tokensOwed0, uint128 tokensOwed1)
func (_NFPM *NFPMCaller) Positions(opts *bind.CallOpts, tokenId *big.Int) (struct {
	Nonce                    *big.Int
	Operator                 common.Address
	Token0                   common.Address
	Token1                   common.Address
	TickSpacing              *big.Int
	TickLower                *big.Int
	TickUpper                *big.Int
	Liquidity                *big.Int
	FeeGrowthInside0LastX128 *big.Int
	FeeGrowthInside1LastX128 *big.Int
	TokensOwed0              *big.Int
	TokensOwed1              *big.Int
}, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "positions", tokenId)

	outstruct := new(struct {
		Nonce                    *big.Int
		Operator                 common.Address
		Token0                   common.Address
		Token1                   common.Address
		TickSpacing              *big.Int
		TickLower                *big.Int
		TickUpper                *big.Int
		Liquidity                *big.Int
		FeeGrowthInside0LastX128 *big.Int
		FeeGrowthInside1LastX128 *big.Int
		TokensOwed0              *big.Int
		TokensOwed1              *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Nonce = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Operator = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.Token0 = *abi.ConvertType(out[2], new(common.Address)).(*common.Address)
	outstruct.Token1 = *abi.ConvertType(out[3], new(common.Address)).(*common.Address)
	outstruct.TickSpacing = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.TickLower = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.TickUpper = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)
	outstruct.Liquidity = *abi.ConvertType(out[7], new(*big.Int)).(**big.Int)
	outstruct.FeeGrowthInside0LastX128 = *abi.ConvertType(out[8], new(*big.Int)).(**big.Int)
	outstruct.FeeGrowthInside1LastX128 = *abi.ConvertType(out[9], new(*big.Int)).(**big.Int)
	outstruct.TokensOwed0 = *abi.ConvertType(out[10], new(*big.Int)).(**big.Int)
	outstruct.TokensOwed1 = *abi.ConvertType(out[11], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// Positions is a free data retrieval call binding the contract method 0x99fbab88.
//
// Solidity: function positions(uint256 tokenId) view returns(uint96 nonce, address operator, address token0, address token1, int24 tickSpacing, int24 tickLower, int24 tickUpper, uint128 liquidity, uint256 feeGrowthInside0LastX128, uint256 feeGrowthInside1LastX128, uint128 tokensOwed0, uint128 tokensOwed1)
func (_NFPM *NFPMSession) Positions(tokenId *big.Int) (struct {
	Nonce                    *big.Int
	Operator                 common.Address
	Token0                   common.Address
	Token1                   common.Address
	TickSpacing              *big.Int
	TickLower                *big.Int
	TickUpper                *big.Int
	Liquidity                *big.Int
	FeeGrowthInside0LastX128 *big.Int
	FeeGrowthInside1LastX128 *big.Int
	TokensOwed0              *big.Int
	TokensOwed1              *big.Int
}, error) {
	return _NFPM.Contract.Positions(&_NFPM.CallOpts, tokenId)
}

// Positions is a free data retrieval call binding the contract method 0x99fbab88.
//
// Solidity: function positions(uint256 tokenId) view returns(uint96 nonce, address operator, address token0, address token1, int24 tickSpacing, int24 tickLower, int24 tickUpper, uint128 liquidity, uint256 feeGrowthInside0LastX128, uint256 feeGrowthInside1LastX128, uint128 tokensOwed0, uint128 tokensOwed1)
func (_NFPM *NFPMCallerSession) Positions(tokenId *big.Int) (struct {
	Nonce                    *big.Int
	Operator                 common.Address
	Token0                   common.Address
	Token1                   common.Address
	TickSpacing              *big.Int
	TickLower                *big.Int
	TickUpper                *big.Int
	Liquidity                *big.Int
	FeeGrowthInside0LastX128 *big.Int
	FeeGrowthInside1LastX128 *big.Int
	TokensOwed0              *big.Int
	TokensOwed1              *big.Int
}, error) {
	return _NFPM.Contract.Positions(&_NFPM.CallOpts, tokenId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_NFPM *NFPMCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_NFPM *NFPMSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _NFPM.Contract.SupportsInterface(&_NFPM.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_NFPM *NFPMCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _NFPM.Contract.SupportsInterface(&_NFPM.CallOpts, interfaceId)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_NFPM *NFPMCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_NFPM *NFPMSession) Symbol() (string, error) {
	return _NFPM.Contract.Symbol(&_NFPM.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_NFPM *NFPMCallerSession) Symbol() (string, error) {
	return _NFPM.Contract.Symbol(&_NFPM.CallOpts)
}

// TokenByIndex is a free data retrieval call binding the contract method 0x4f6ccce7.
//
// Solidity: function tokenByIndex(uint256 index) view returns(uint256)
func (_NFPM *NFPMCaller) TokenByIndex(opts *bind.CallOpts, index *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "tokenByIndex", index)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TokenByIndex is a free data retrieval call binding the contract method 0x4f6ccce7.
//
// Solidity: function tokenByIndex(uint256 index) view returns(uint256)
func (_NFPM *NFPMSession) TokenByIndex(index *big.Int) (*big.Int, error) {
	return _NFPM.Contract.TokenByIndex(&_NFPM.CallOpts, index)
}

// TokenByIndex is a free data retrieval call binding the contract method 0x4f6ccce7.
//
// Solidity: function tokenByIndex(uint256 index) view returns(uint256)
func (_NFPM *NFPMCallerSession) TokenByIndex(index *big.Int) (*big.Int, error) {
	return _NFPM.Contract.TokenByIndex(&_NFPM.CallOpts, index)
}

// TokenDescriptor is a free data retrieval call binding the contract method 0x5a9d7a68.
//
// Solidity: function tokenDescriptor() view returns(address)
func (_NFPM *NFPMCaller) TokenDescriptor(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "tokenDescriptor")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TokenDescriptor is a free data retrieval call binding the contract method 0x5a9d7a68.
//
// Solidity: function tokenDescriptor() view returns(address)
func (_NFPM *NFPMSession) TokenDescriptor() (common.Address, error) {
	return _NFPM.Contract.TokenDescriptor(&_NFPM.CallOpts)
}

// TokenDescriptor is a free data retrieval call binding the contract method 0x5a9d7a68.
//
// Solidity: function tokenDescriptor() view returns(address)
func (_NFPM *NFPMCallerSession) TokenDescriptor() (common.Address, error) {
	return _NFPM.Contract.TokenDescriptor(&_NFPM.CallOpts)
}

// TokenOfOwnerByIndex is a free data retrieval call binding the contract method 0x2f745c59.
//
// Solidity: function tokenOfOwnerByIndex(address owner, uint256 index) view returns(uint256)
func (_NFPM *NFPMCaller) TokenOfOwnerByIndex(opts *bind.CallOpts, owner common.Address, index *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "tokenOfOwnerByIndex", owner, index)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TokenOfOwnerByIndex is a free data retrieval call binding the contract method 0x2f745c59.
//
// Solidity: function tokenOfOwnerByIndex(address owner, uint256 index) view returns(uint256)
func (_NFPM *NFPMSession) TokenOfOwnerByIndex(owner common.Address, index *big.Int) (*big.Int, error) {
	return _NFPM.Contract.TokenOfOwnerByIndex(&_NFPM.CallOpts, owner, index)
}

// TokenOfOwnerByIndex is a free data retrieval call binding the contract method 0x2f745c59.
//
// Solidity: function tokenOfOwnerByIndex(address owner, uint256 index) view returns(uint256)
func (_NFPM *NFPMCallerSession) TokenOfOwnerByIndex(owner common.Address, index *big.Int) (*big.Int, error) {
	return _NFPM.Contract.TokenOfOwnerByIndex(&_NFPM.CallOpts, owner, index)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_NFPM *NFPMCaller) TokenURI(opts *bind.CallOpts, tokenId *big.Int) (string, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "tokenURI", tokenId)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_NFPM *NFPMSession) TokenURI(tokenId *big.Int) (string, error) {
	return _NFPM.Contract.TokenURI(&_NFPM.CallOpts, tokenId)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 tokenId) view returns(string)
func (_NFPM *NFPMCallerSession) TokenURI(tokenId *big.Int) (string, error) {
	return _NFPM.Contract.TokenURI(&_NFPM.CallOpts, tokenId)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_NFPM *NFPMCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _NFPM.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_NFPM *NFPMSession) TotalSupply() (*big.Int, error) {
	return _NFPM.Contract.TotalSupply(&_NFPM.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_NFPM *NFPMCallerSession) TotalSupply() (*big.Int, error) {
	return _NFPM.Contract.TotalSupply(&_NFPM.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_NFPM *NFPMTransactor) Approve(opts *bind.TransactOpts, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "approve", to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_NFPM *NFPMSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.Contract.Approve(&_NFPM.TransactOpts, to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_NFPM *NFPMTransactorSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.Contract.Approve(&_NFPM.TransactOpts, to, tokenId)
}

// Burn is a paid mutator transaction binding the contract method 0x42966c68.
//
// Solidity: function burn(uint256 tokenId) payable returns()
func (_NFPM *NFPMTransactor) Burn(opts *bind.TransactOpts, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "burn", tokenId)
}

// Burn is a paid mutator transaction binding the contract method 0x42966c68.
//
// Solidity: function burn(uint256 tokenId) payable returns()
func (_NFPM *NFPMSession) Burn(tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.Contract.Burn(&_NFPM.TransactOpts, tokenId)
}

// Burn is a paid mutator transaction binding the contract method 0x42966c68.
//
// Solidity: function burn(uint256 tokenId) payable returns()
func (_NFPM *NFPMTransactorSession) Burn(tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.Contract.Burn(&_NFPM.TransactOpts, tokenId)
}

// Collect is a paid mutator transaction binding the contract method 0xfc6f7865.
//
// Solidity: function collect((uint256,address,uint128,uint128) params) payable returns(uint256 amount0, uint256 amount1)
func (_NFPM *NFPMTransactor) Collect(opts *bind.TransactOpts, params INonfungiblePositionManagerCollectParams) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "collect", params)
}

// Collect is a paid mutator transaction binding the contract method 0xfc6f7865.
//
// Solidity: function collect((uint256,address,uint128,uint128) params) payable returns(uint256 amount0, uint256 amount1)
func (_NFPM *NFPMSession) Collect(params INonfungiblePositionManagerCollectParams) (*types.Transaction, error) {
	return _NFPM.Contract.Collect(&_NFPM.TransactOpts, params)
}

// Collect is a paid mutator transaction binding the contract method 0xfc6f7865.
//
// Solidity: function collect((uint256,address,uint128,uint128) params) payable returns(uint256 amount0, uint256 amount1)
func (_NFPM *NFPMTransactorSession) Collect(params INonfungiblePositionManagerCollectParams) (*types.Transaction, error) {
	return _NFPM.Contract.Collect(&_NFPM.TransactOpts, params)
}

// DecreaseLiquidity is a paid mutator transaction binding the contract method 0x0c49ccbe.
//
// Solidity: function decreaseLiquidity((uint256,uint128,uint256,uint256,uint256) params) payable returns(uint256 amount0, uint256 amount1)
func (_NFPM *NFPMTransactor) DecreaseLiquidity(opts *bind.TransactOpts, params INonfungiblePositionManagerDecreaseLiquidityParams) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "decreaseLiquidity", params)
}

// DecreaseLiquidity is a paid mutator transaction binding the contract method 0x0c49ccbe.
//
// Solidity: function decreaseLiquidity((uint256,uint128,uint256,uint256,uint256) params) payable returns(uint256 amount0, uint256 amount1)
func (_NFPM *NFPMSession) DecreaseLiquidity(params INonfungiblePositionManagerDecreaseLiquidityParams) (*types.Transaction, error) {
	return _NFPM.Contract.DecreaseLiquidity(&_NFPM.TransactOpts, params)
}

// DecreaseLiquidity is a paid mutator transaction binding the contract method 0x0c49ccbe.
//
// Solidity: function decreaseLiquidity((uint256,uint128,uint256,uint256,uint256) params) payable returns(uint256 amount0, uint256 amount1)
func (_NFPM *NFPMTransactorSession) DecreaseLiquidity(params INonfungiblePositionManagerDecreaseLiquidityParams) (*types.Transaction, error) {
	return _NFPM.Contract.DecreaseLiquidity(&_NFPM.TransactOpts, params)
}

// IncreaseLiquidity is a paid mutator transaction binding the contract method 0x219f5d17.
//
// Solidity: function increaseLiquidity((uint256,uint256,uint256,uint256,uint256,uint256) params) payable returns(uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMTransactor) IncreaseLiquidity(opts *bind.TransactOpts, params INonfungiblePositionManagerIncreaseLiquidityParams) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "increaseLiquidity", params)
}

// IncreaseLiquidity is a paid mutator transaction binding the contract method 0x219f5d17.
//
// Solidity: function increaseLiquidity((uint256,uint256,uint256,uint256,uint256,uint256) params) payable returns(uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMSession) IncreaseLiquidity(params INonfungiblePositionManagerIncreaseLiquidityParams) (*types.Transaction, error) {
	return _NFPM.Contract.IncreaseLiquidity(&_NFPM.TransactOpts, params)
}

// IncreaseLiquidity is a paid mutator transaction binding the contract method 0x219f5d17.
//
// Solidity: function increaseLiquidity((uint256,uint256,uint256,uint256,uint256,uint256) params) payable returns(uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMTransactorSession) IncreaseLiquidity(params INonfungiblePositionManagerIncreaseLiquidityParams) (*types.Transaction, error) {
	return _NFPM.Contract.IncreaseLiquidity(&_NFPM.TransactOpts, params)
}

// Mint is a paid mutator transaction binding the contract method 0xb5007d1f.
//
// Solidity: function mint((address,address,int24,int24,int24,uint256,uint256,uint256,uint256,address,uint256,uint160) params) payable returns(uint256 tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMTransactor) Mint(opts *bind.TransactOpts, params INonfungiblePositionManagerMintParams) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "mint", params)
}

// Mint is a paid mutator transaction binding the contract method 0xb5007d1f.
//
// Solidity: function mint((address,address,int24,int24,int24,uint256,uint256,uint256,uint256,address,uint256,uint160) params) payable returns(uint256 tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMSession) Mint(params INonfungiblePositionManagerMintParams) (*types.Transaction, error) {
	return _NFPM.Contract.Mint(&_NFPM.TransactOpts, params)
}

// Mint is a paid mutator transaction binding the contract method 0xb5007d1f.
//
// Solidity: function mint((address,address,int24,int24,int24,uint256,uint256,uint256,uint256,address,uint256,uint160) params) payable returns(uint256 tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMTransactorSession) Mint(params INonfungiblePositionManagerMintParams) (*types.Transaction, error) {
	return _NFPM.Contract.Mint(&_NFPM.TransactOpts, params)
}

// Multicall is a paid mutator transaction binding the contract method 0xac9650d8.
//
// Solidity: function multicall(bytes[] data) payable returns(bytes[] results)
func (_NFPM *NFPMTransactor) Multicall(opts *bind.TransactOpts, data [][]byte) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "multicall", data)
}

// Multicall is a paid mutator transaction binding the contract method 0xac9650d8.
//
// Solidity: function multicall(bytes[] data) payable returns(bytes[] results)
func (_NFPM *NFPMSession) Multicall(data [][]byte) (*types.Transaction, error) {
	return _NFPM.Contract.Multicall(&_NFPM.TransactOpts, data)
}

// Multicall is a paid mutator transaction binding the contract method 0xac9650d8.
//
// Solidity: function multicall(bytes[] data) payable returns(bytes[] results)
func (_NFPM *NFPMTransactorSession) Multicall(data [][]byte) (*types.Transaction, error) {
	return _NFPM.Contract.Multicall(&_NFPM.TransactOpts, data)
}

// Permit is a paid mutator transaction binding the contract method 0x7ac2ff7b.
//
// Solidity: function permit(address spender, uint256 tokenId, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactor) Permit(opts *bind.TransactOpts, spender common.Address, tokenId *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "permit", spender, tokenId, deadline, v, r, s)
}

// Permit is a paid mutator transaction binding the contract method 0x7ac2ff7b.
//
// Solidity: function permit(address spender, uint256 tokenId, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMSession) Permit(spender common.Address, tokenId *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.Permit(&_NFPM.TransactOpts, spender, tokenId, deadline, v, r, s)
}

// Permit is a paid mutator transaction binding the contract method 0x7ac2ff7b.
//
// Solidity: function permit(address spender, uint256 tokenId, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactorSession) Permit(spender common.Address, tokenId *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.Permit(&_NFPM.TransactOpts, spender, tokenId, deadline, v, r, s)
}

// RefundETH is a paid mutator transaction binding the contract method 0x12210e8a.
//
// Solidity: function refundETH() payable returns()
func (_NFPM *NFPMTransactor) RefundETH(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "refundETH")
}

// RefundETH is a paid mutator transaction binding the contract method 0x12210e8a.
//
// Solidity: function refundETH() payable returns()
func (_NFPM *NFPMSession) RefundETH() (*types.Transaction, error) {
	return _NFPM.Contract.RefundETH(&_NFPM.TransactOpts)
}

// RefundETH is a paid mutator transaction binding the contract method 0x12210e8a.
//
// Solidity: function refundETH() payable returns()
func (_NFPM *NFPMTransactorSession) RefundETH() (*types.Transaction, error) {
	return _NFPM.Contract.RefundETH(&_NFPM.TransactOpts)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_NFPM *NFPMTransactor) SafeTransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "safeTransferFrom", from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_NFPM *NFPMSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.Contract.SafeTransferFrom(&_NFPM.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_NFPM *NFPMTransactorSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.Contract.SafeTransferFrom(&_NFPM.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes _data) returns()
func (_NFPM *NFPMTransactor) SafeTransferFrom0(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int, _data []byte) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "safeTransferFrom0", from, to, tokenId, _data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes _data) returns()
func (_NFPM *NFPMSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, _data []byte) (*types.Transaction, error) {
	return _NFPM.Contract.SafeTransferFrom0(&_NFPM.TransactOpts, from, to, tokenId, _data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes _data) returns()
func (_NFPM *NFPMTransactorSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, _data []byte) (*types.Transaction, error) {
	return _NFPM.Contract.SafeTransferFrom0(&_NFPM.TransactOpts, from, to, tokenId, _data)
}

// SelfPermit is a paid mutator transaction binding the contract method 0xf3995c67.
//
// Solidity: function selfPermit(address token, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactor) SelfPermit(opts *bind.TransactOpts, token common.Address, value *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "selfPermit", token, value, deadline, v, r, s)
}

// SelfPermit is a paid mutator transaction binding the contract method 0xf3995c67.
//
// Solidity: function selfPermit(address token, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMSession) SelfPermit(token common.Address, value *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.SelfPermit(&_NFPM.TransactOpts, token, value, deadline, v, r, s)
}

// SelfPermit is a paid mutator transaction binding the contract method 0xf3995c67.
//
// Solidity: function selfPermit(address token, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactorSession) SelfPermit(token common.Address, value *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.SelfPermit(&_NFPM.TransactOpts, token, value, deadline, v, r, s)
}

// SelfPermitAllowed is a paid mutator transaction binding the contract method 0x4659a494.
//
// Solidity: function selfPermitAllowed(address token, uint256 nonce, uint256 expiry, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactor) SelfPermitAllowed(opts *bind.TransactOpts, token common.Address, nonce *big.Int, expiry *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "selfPermitAllowed", token, nonce, expiry, v, r, s)
}

// SelfPermitAllowed is a paid mutator transaction binding the contract method 0x4659a494.
//
// Solidity: function selfPermitAllowed(address token, uint256 nonce, uint256 expiry, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMSession) SelfPermitAllowed(token common.Address, nonce *big.Int, expiry *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.SelfPermitAllowed(&_NFPM.TransactOpts, token, nonce, expiry, v, r, s)
}

// SelfPermitAllowed is a paid mutator transaction binding the contract method 0x4659a494.
//
// Solidity: function selfPermitAllowed(address token, uint256 nonce, uint256 expiry, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactorSession) SelfPermitAllowed(token common.Address, nonce *big.Int, expiry *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.SelfPermitAllowed(&_NFPM.TransactOpts, token, nonce, expiry, v, r, s)
}

// SelfPermitAllowedIfNecessary is a paid mutator transaction binding the contract method 0xa4a78f0c.
//
// Solidity: function selfPermitAllowedIfNecessary(address token, uint256 nonce, uint256 expiry, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactor) SelfPermitAllowedIfNecessary(opts *bind.TransactOpts, token common.Address, nonce *big.Int, expiry *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "selfPermitAllowedIfNecessary", token, nonce, expiry, v, r, s)
}

// SelfPermitAllowedIfNecessary is a paid mutator transaction binding the contract method 0xa4a78f0c.
//
// Solidity: function selfPermitAllowedIfNecessary(address token, uint256 nonce, uint256 expiry, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMSession) SelfPermitAllowedIfNecessary(token common.Address, nonce *big.Int, expiry *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.SelfPermitAllowedIfNecessary(&_NFPM.TransactOpts, token, nonce, expiry, v, r, s)
}

// SelfPermitAllowedIfNecessary is a paid mutator transaction binding the contract method 0xa4a78f0c.
//
// Solidity: function selfPermitAllowedIfNecessary(address token, uint256 nonce, uint256 expiry, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactorSession) SelfPermitAllowedIfNecessary(token common.Address, nonce *big.Int, expiry *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.SelfPermitAllowedIfNecessary(&_NFPM.TransactOpts, token, nonce, expiry, v, r, s)
}

// SelfPermitIfNecessary is a paid mutator transaction binding the contract method 0xc2e3140a.
//
// Solidity: function selfPermitIfNecessary(address token, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactor) SelfPermitIfNecessary(opts *bind.TransactOpts, token common.Address, value *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "selfPermitIfNecessary", token, value, deadline, v, r, s)
}

// SelfPermitIfNecessary is a paid mutator transaction binding the contract method 0xc2e3140a.
//
// Solidity: function selfPermitIfNecessary(address token, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMSession) SelfPermitIfNecessary(token common.Address, value *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.SelfPermitIfNecessary(&_NFPM.TransactOpts, token, value, deadline, v, r, s)
}

// SelfPermitIfNecessary is a paid mutator transaction binding the contract method 0xc2e3140a.
//
// Solidity: function selfPermitIfNecessary(address token, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) payable returns()
func (_NFPM *NFPMTransactorSession) SelfPermitIfNecessary(token common.Address, value *big.Int, deadline *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _NFPM.Contract.SelfPermitIfNecessary(&_NFPM.TransactOpts, token, value, deadline, v, r, s)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_NFPM *NFPMTransactor) SetApprovalForAll(opts *bind.TransactOpts, operator common.Address, approved bool) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "setApprovalForAll", operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_NFPM *NFPMSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _NFPM.Contract.SetApprovalForAll(&_NFPM.TransactOpts, operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_NFPM *NFPMTransactorSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _NFPM.Contract.SetApprovalForAll(&_NFPM.TransactOpts, operator, approved)
}

// SetOwner is a paid mutator transaction binding the contract method 0x13af4035.
//
// Solidity: function setOwner(address _owner) returns()
func (_NFPM *NFPMTransactor) SetOwner(opts *bind.TransactOpts, _owner common.Address) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "setOwner", _owner)
}

// SetOwner is a paid mutator transaction binding the contract method 0x13af4035.
//
// Solidity: function setOwner(address _owner) returns()
func (_NFPM *NFPMSession) SetOwner(_owner common.Address) (*types.Transaction, error) {
	return _NFPM.Contract.SetOwner(&_NFPM.TransactOpts, _owner)
}

// SetOwner is a paid mutator transaction binding the contract method 0x13af4035.
//
// Solidity: function setOwner(address _owner) returns()
func (_NFPM *NFPMTransactorSession) SetOwner(_owner common.Address) (*types.Transaction, error) {
	return _NFPM.Contract.SetOwner(&_NFPM.TransactOpts, _owner)
}

// SetTokenDescriptor is a paid mutator transaction binding the contract method 0xb6dc7ea3.
//
// Solidity: function setTokenDescriptor(address _tokenDescriptor) returns()
func (_NFPM *NFPMTransactor) SetTokenDescriptor(opts *bind.TransactOpts, _tokenDescriptor common.Address) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "setTokenDescriptor", _tokenDescriptor)
}

// SetTokenDescriptor is a paid mutator transaction binding the contract method 0xb6dc7ea3.
//
// Solidity: function setTokenDescriptor(address _tokenDescriptor) returns()
func (_NFPM *NFPMSession) SetTokenDescriptor(_tokenDescriptor common.Address) (*types.Transaction, error) {
	return _NFPM.Contract.SetTokenDescriptor(&_NFPM.TransactOpts, _tokenDescriptor)
}

// SetTokenDescriptor is a paid mutator transaction binding the contract method 0xb6dc7ea3.
//
// Solidity: function setTokenDescriptor(address _tokenDescriptor) returns()
func (_NFPM *NFPMTransactorSession) SetTokenDescriptor(_tokenDescriptor common.Address) (*types.Transaction, error) {
	return _NFPM.Contract.SetTokenDescriptor(&_NFPM.TransactOpts, _tokenDescriptor)
}

// SweepToken is a paid mutator transaction binding the contract method 0xdf2ab5bb.
//
// Solidity: function sweepToken(address token, uint256 amountMinimum, address recipient) payable returns()
func (_NFPM *NFPMTransactor) SweepToken(opts *bind.TransactOpts, token common.Address, amountMinimum *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "sweepToken", token, amountMinimum, recipient)
}

// SweepToken is a paid mutator transaction binding the contract method 0xdf2ab5bb.
//
// Solidity: function sweepToken(address token, uint256 amountMinimum, address recipient) payable returns()
func (_NFPM *NFPMSession) SweepToken(token common.Address, amountMinimum *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _NFPM.Contract.SweepToken(&_NFPM.TransactOpts, token, amountMinimum, recipient)
}

// SweepToken is a paid mutator transaction binding the contract method 0xdf2ab5bb.
//
// Solidity: function sweepToken(address token, uint256 amountMinimum, address recipient) payable returns()
func (_NFPM *NFPMTransactorSession) SweepToken(token common.Address, amountMinimum *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _NFPM.Contract.SweepToken(&_NFPM.TransactOpts, token, amountMinimum, recipient)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_NFPM *NFPMTransactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "transferFrom", from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_NFPM *NFPMSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.Contract.TransferFrom(&_NFPM.TransactOpts, from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_NFPM *NFPMTransactorSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NFPM.Contract.TransferFrom(&_NFPM.TransactOpts, from, to, tokenId)
}

// UniswapV3MintCallback is a paid mutator transaction binding the contract method 0xd3487997.
//
// Solidity: function uniswapV3MintCallback(uint256 amount0Owed, uint256 amount1Owed, bytes data) returns()
func (_NFPM *NFPMTransactor) UniswapV3MintCallback(opts *bind.TransactOpts, amount0Owed *big.Int, amount1Owed *big.Int, data []byte) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "uniswapV3MintCallback", amount0Owed, amount1Owed, data)
}

// UniswapV3MintCallback is a paid mutator transaction binding the contract method 0xd3487997.
//
// Solidity: function uniswapV3MintCallback(uint256 amount0Owed, uint256 amount1Owed, bytes data) returns()
func (_NFPM *NFPMSession) UniswapV3MintCallback(amount0Owed *big.Int, amount1Owed *big.Int, data []byte) (*types.Transaction, error) {
	return _NFPM.Contract.UniswapV3MintCallback(&_NFPM.TransactOpts, amount0Owed, amount1Owed, data)
}

// UniswapV3MintCallback is a paid mutator transaction binding the contract method 0xd3487997.
//
// Solidity: function uniswapV3MintCallback(uint256 amount0Owed, uint256 amount1Owed, bytes data) returns()
func (_NFPM *NFPMTransactorSession) UniswapV3MintCallback(amount0Owed *big.Int, amount1Owed *big.Int, data []byte) (*types.Transaction, error) {
	return _NFPM.Contract.UniswapV3MintCallback(&_NFPM.TransactOpts, amount0Owed, amount1Owed, data)
}

// UnwrapWETH9 is a paid mutator transaction binding the contract method 0x49404b7c.
//
// Solidity: function unwrapWETH9(uint256 amountMinimum, address recipient) payable returns()
func (_NFPM *NFPMTransactor) UnwrapWETH9(opts *bind.TransactOpts, amountMinimum *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _NFPM.contract.Transact(opts, "unwrapWETH9", amountMinimum, recipient)
}

// UnwrapWETH9 is a paid mutator transaction binding the contract method 0x49404b7c.
//
// Solidity: function unwrapWETH9(uint256 amountMinimum, address recipient) payable returns()
func (_NFPM *NFPMSession) UnwrapWETH9(amountMinimum *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _NFPM.Contract.UnwrapWETH9(&_NFPM.TransactOpts, amountMinimum, recipient)
}

// UnwrapWETH9 is a paid mutator transaction binding the contract method 0x49404b7c.
//
// Solidity: function unwrapWETH9(uint256 amountMinimum, address recipient) payable returns()
func (_NFPM *NFPMTransactorSession) UnwrapWETH9(amountMinimum *big.Int, recipient common.Address) (*types.Transaction, error) {
	return _NFPM.Contract.UnwrapWETH9(&_NFPM.TransactOpts, amountMinimum, recipient)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_NFPM *NFPMTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NFPM.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_NFPM *NFPMSession) Receive() (*types.Transaction, error) {
	return _NFPM.Contract.Receive(&_NFPM.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_NFPM *NFPMTransactorSession) Receive() (*types.Transaction, error) {
	return _NFPM.Contract.Receive(&_NFPM.TransactOpts)
}

// NFPMApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the NFPM contract.
type NFPMApprovalIterator struct {
	Event *NFPMApproval // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMApproval)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMApproval)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMApproval represents a Approval event raised by the NFPM contract.
type NFPMApproval struct {
	Owner    common.Address
	Approved common.Address
	TokenId  *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_NFPM *NFPMFilterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, approved []common.Address, tokenId []*big.Int) (*NFPMApprovalIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var approvedRule []interface{}
	for _, approvedItem := range approved {
		approvedRule = append(approvedRule, approvedItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &NFPMApprovalIterator{contract: _NFPM.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_NFPM *NFPMFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *NFPMApproval, owner []common.Address, approved []common.Address, tokenId []*big.Int) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var approvedRule []interface{}
	for _, approvedItem := range approved {
		approvedRule = append(approvedRule, approvedItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMApproval)
				if err := _NFPM.contract.UnpackLog(event, "Approval", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseApproval is a log parse operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_NFPM *NFPMFilterer) ParseApproval(log types.Log) (*NFPMApproval, error) {
	event := new(NFPMApproval)
	if err := _NFPM.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMApprovalForAllIterator is returned from FilterApprovalForAll and is used to iterate over the raw logs and unpacked data for ApprovalForAll events raised by the NFPM contract.
type NFPMApprovalForAllIterator struct {
	Event *NFPMApprovalForAll // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMApprovalForAllIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMApprovalForAll)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMApprovalForAll)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMApprovalForAllIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMApprovalForAllIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMApprovalForAll represents a ApprovalForAll event raised by the NFPM contract.
type NFPMApprovalForAll struct {
	Owner    common.Address
	Operator common.Address
	Approved bool
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApprovalForAll is a free log retrieval operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_NFPM *NFPMFilterer) FilterApprovalForAll(opts *bind.FilterOpts, owner []common.Address, operator []common.Address) (*NFPMApprovalForAllIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &NFPMApprovalForAllIterator{contract: _NFPM.contract, event: "ApprovalForAll", logs: logs, sub: sub}, nil
}

// WatchApprovalForAll is a free log subscription operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_NFPM *NFPMFilterer) WatchApprovalForAll(opts *bind.WatchOpts, sink chan<- *NFPMApprovalForAll, owner []common.Address, operator []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMApprovalForAll)
				if err := _NFPM.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseApprovalForAll is a log parse operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_NFPM *NFPMFilterer) ParseApprovalForAll(log types.Log) (*NFPMApprovalForAll, error) {
	event := new(NFPMApprovalForAll)
	if err := _NFPM.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMBatchMetadataUpdateIterator is returned from FilterBatchMetadataUpdate and is used to iterate over the raw logs and unpacked data for BatchMetadataUpdate events raised by the NFPM contract.
type NFPMBatchMetadataUpdateIterator struct {
	Event *NFPMBatchMetadataUpdate // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMBatchMetadataUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMBatchMetadataUpdate)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMBatchMetadataUpdate)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMBatchMetadataUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMBatchMetadataUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMBatchMetadataUpdate represents a BatchMetadataUpdate event raised by the NFPM contract.
type NFPMBatchMetadataUpdate struct {
	FromTokenId *big.Int
	ToTokenId   *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterBatchMetadataUpdate is a free log retrieval operation binding the contract event 0x6bd5c950a8d8df17f772f5af37cb3655737899cbf903264b9795592da439661c.
//
// Solidity: event BatchMetadataUpdate(uint256 _fromTokenId, uint256 _toTokenId)
func (_NFPM *NFPMFilterer) FilterBatchMetadataUpdate(opts *bind.FilterOpts) (*NFPMBatchMetadataUpdateIterator, error) {

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "BatchMetadataUpdate")
	if err != nil {
		return nil, err
	}
	return &NFPMBatchMetadataUpdateIterator{contract: _NFPM.contract, event: "BatchMetadataUpdate", logs: logs, sub: sub}, nil
}

// WatchBatchMetadataUpdate is a free log subscription operation binding the contract event 0x6bd5c950a8d8df17f772f5af37cb3655737899cbf903264b9795592da439661c.
//
// Solidity: event BatchMetadataUpdate(uint256 _fromTokenId, uint256 _toTokenId)
func (_NFPM *NFPMFilterer) WatchBatchMetadataUpdate(opts *bind.WatchOpts, sink chan<- *NFPMBatchMetadataUpdate) (event.Subscription, error) {

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "BatchMetadataUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMBatchMetadataUpdate)
				if err := _NFPM.contract.UnpackLog(event, "BatchMetadataUpdate", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseBatchMetadataUpdate is a log parse operation binding the contract event 0x6bd5c950a8d8df17f772f5af37cb3655737899cbf903264b9795592da439661c.
//
// Solidity: event BatchMetadataUpdate(uint256 _fromTokenId, uint256 _toTokenId)
func (_NFPM *NFPMFilterer) ParseBatchMetadataUpdate(log types.Log) (*NFPMBatchMetadataUpdate, error) {
	event := new(NFPMBatchMetadataUpdate)
	if err := _NFPM.contract.UnpackLog(event, "BatchMetadataUpdate", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMCollectIterator is returned from FilterCollect and is used to iterate over the raw logs and unpacked data for Collect events raised by the NFPM contract.
type NFPMCollectIterator struct {
	Event *NFPMCollect // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMCollectIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMCollect)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMCollect)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMCollectIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMCollectIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMCollect represents a Collect event raised by the NFPM contract.
type NFPMCollect struct {
	TokenId   *big.Int
	Recipient common.Address
	Amount0   *big.Int
	Amount1   *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterCollect is a free log retrieval operation binding the contract event 0x40d0efd1a53d60ecbf40971b9daf7dc90178c3aadc7aab1765632738fa8b8f01.
//
// Solidity: event Collect(uint256 indexed tokenId, address recipient, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) FilterCollect(opts *bind.FilterOpts, tokenId []*big.Int) (*NFPMCollectIterator, error) {

	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "Collect", tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &NFPMCollectIterator{contract: _NFPM.contract, event: "Collect", logs: logs, sub: sub}, nil
}

// WatchCollect is a free log subscription operation binding the contract event 0x40d0efd1a53d60ecbf40971b9daf7dc90178c3aadc7aab1765632738fa8b8f01.
//
// Solidity: event Collect(uint256 indexed tokenId, address recipient, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) WatchCollect(opts *bind.WatchOpts, sink chan<- *NFPMCollect, tokenId []*big.Int) (event.Subscription, error) {

	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "Collect", tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMCollect)
				if err := _NFPM.contract.UnpackLog(event, "Collect", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCollect is a log parse operation binding the contract event 0x40d0efd1a53d60ecbf40971b9daf7dc90178c3aadc7aab1765632738fa8b8f01.
//
// Solidity: event Collect(uint256 indexed tokenId, address recipient, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) ParseCollect(log types.Log) (*NFPMCollect, error) {
	event := new(NFPMCollect)
	if err := _NFPM.contract.UnpackLog(event, "Collect", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMDecreaseLiquidityIterator is returned from FilterDecreaseLiquidity and is used to iterate over the raw logs and unpacked data for DecreaseLiquidity events raised by the NFPM contract.
type NFPMDecreaseLiquidityIterator struct {
	Event *NFPMDecreaseLiquidity // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMDecreaseLiquidityIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMDecreaseLiquidity)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMDecreaseLiquidity)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMDecreaseLiquidityIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMDecreaseLiquidityIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMDecreaseLiquidity represents a DecreaseLiquidity event raised by the NFPM contract.
type NFPMDecreaseLiquidity struct {
	TokenId   *big.Int
	Liquidity *big.Int
	Amount0   *big.Int
	Amount1   *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterDecreaseLiquidity is a free log retrieval operation binding the contract event 0x26f6a048ee9138f2c0ce266f322cb99228e8d619ae2bff30c67f8dcf9d2377b4.
//
// Solidity: event DecreaseLiquidity(uint256 indexed tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) FilterDecreaseLiquidity(opts *bind.FilterOpts, tokenId []*big.Int) (*NFPMDecreaseLiquidityIterator, error) {

	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "DecreaseLiquidity", tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &NFPMDecreaseLiquidityIterator{contract: _NFPM.contract, event: "DecreaseLiquidity", logs: logs, sub: sub}, nil
}

// WatchDecreaseLiquidity is a free log subscription operation binding the contract event 0x26f6a048ee9138f2c0ce266f322cb99228e8d619ae2bff30c67f8dcf9d2377b4.
//
// Solidity: event DecreaseLiquidity(uint256 indexed tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) WatchDecreaseLiquidity(opts *bind.WatchOpts, sink chan<- *NFPMDecreaseLiquidity, tokenId []*big.Int) (event.Subscription, error) {

	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "DecreaseLiquidity", tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMDecreaseLiquidity)
				if err := _NFPM.contract.UnpackLog(event, "DecreaseLiquidity", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDecreaseLiquidity is a log parse operation binding the contract event 0x26f6a048ee9138f2c0ce266f322cb99228e8d619ae2bff30c67f8dcf9d2377b4.
//
// Solidity: event DecreaseLiquidity(uint256 indexed tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) ParseDecreaseLiquidity(log types.Log) (*NFPMDecreaseLiquidity, error) {
	event := new(NFPMDecreaseLiquidity)
	if err := _NFPM.contract.UnpackLog(event, "DecreaseLiquidity", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMIncreaseLiquidityIterator is returned from FilterIncreaseLiquidity and is used to iterate over the raw logs and unpacked data for IncreaseLiquidity events raised by the NFPM contract.
type NFPMIncreaseLiquidityIterator struct {
	Event *NFPMIncreaseLiquidity // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMIncreaseLiquidityIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMIncreaseLiquidity)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMIncreaseLiquidity)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMIncreaseLiquidityIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMIncreaseLiquidityIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMIncreaseLiquidity represents a IncreaseLiquidity event raised by the NFPM contract.
type NFPMIncreaseLiquidity struct {
	TokenId   *big.Int
	Liquidity *big.Int
	Amount0   *big.Int
	Amount1   *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterIncreaseLiquidity is a free log retrieval operation binding the contract event 0x3067048beee31b25b2f1681f88dac838c8bba36af25bfb2b7cf7473a5847e35f.
//
// Solidity: event IncreaseLiquidity(uint256 indexed tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) FilterIncreaseLiquidity(opts *bind.FilterOpts, tokenId []*big.Int) (*NFPMIncreaseLiquidityIterator, error) {

	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "IncreaseLiquidity", tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &NFPMIncreaseLiquidityIterator{contract: _NFPM.contract, event: "IncreaseLiquidity", logs: logs, sub: sub}, nil
}

// WatchIncreaseLiquidity is a free log subscription operation binding the contract event 0x3067048beee31b25b2f1681f88dac838c8bba36af25bfb2b7cf7473a5847e35f.
//
// Solidity: event IncreaseLiquidity(uint256 indexed tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) WatchIncreaseLiquidity(opts *bind.WatchOpts, sink chan<- *NFPMIncreaseLiquidity, tokenId []*big.Int) (event.Subscription, error) {

	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "IncreaseLiquidity", tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMIncreaseLiquidity)
				if err := _NFPM.contract.UnpackLog(event, "IncreaseLiquidity", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseIncreaseLiquidity is a log parse operation binding the contract event 0x3067048beee31b25b2f1681f88dac838c8bba36af25bfb2b7cf7473a5847e35f.
//
// Solidity: event IncreaseLiquidity(uint256 indexed tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)
func (_NFPM *NFPMFilterer) ParseIncreaseLiquidity(log types.Log) (*NFPMIncreaseLiquidity, error) {
	event := new(NFPMIncreaseLiquidity)
	if err := _NFPM.contract.UnpackLog(event, "IncreaseLiquidity", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMMetadataUpdateIterator is returned from FilterMetadataUpdate and is used to iterate over the raw logs and unpacked data for MetadataUpdate events raised by the NFPM contract.
type NFPMMetadataUpdateIterator struct {
	Event *NFPMMetadataUpdate // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMMetadataUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMMetadataUpdate)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMMetadataUpdate)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMMetadataUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMMetadataUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMMetadataUpdate represents a MetadataUpdate event raised by the NFPM contract.
type NFPMMetadataUpdate struct {
	TokenId *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterMetadataUpdate is a free log retrieval operation binding the contract event 0xf8e1a15aba9398e019f0b49df1a4fde98ee17ae345cb5f6b5e2c27f5033e8ce7.
//
// Solidity: event MetadataUpdate(uint256 _tokenId)
func (_NFPM *NFPMFilterer) FilterMetadataUpdate(opts *bind.FilterOpts) (*NFPMMetadataUpdateIterator, error) {

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "MetadataUpdate")
	if err != nil {
		return nil, err
	}
	return &NFPMMetadataUpdateIterator{contract: _NFPM.contract, event: "MetadataUpdate", logs: logs, sub: sub}, nil
}

// WatchMetadataUpdate is a free log subscription operation binding the contract event 0xf8e1a15aba9398e019f0b49df1a4fde98ee17ae345cb5f6b5e2c27f5033e8ce7.
//
// Solidity: event MetadataUpdate(uint256 _tokenId)
func (_NFPM *NFPMFilterer) WatchMetadataUpdate(opts *bind.WatchOpts, sink chan<- *NFPMMetadataUpdate) (event.Subscription, error) {

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "MetadataUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMMetadataUpdate)
				if err := _NFPM.contract.UnpackLog(event, "MetadataUpdate", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseMetadataUpdate is a log parse operation binding the contract event 0xf8e1a15aba9398e019f0b49df1a4fde98ee17ae345cb5f6b5e2c27f5033e8ce7.
//
// Solidity: event MetadataUpdate(uint256 _tokenId)
func (_NFPM *NFPMFilterer) ParseMetadataUpdate(log types.Log) (*NFPMMetadataUpdate, error) {
	event := new(NFPMMetadataUpdate)
	if err := _NFPM.contract.UnpackLog(event, "MetadataUpdate", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMTokenDescriptorChangedIterator is returned from FilterTokenDescriptorChanged and is used to iterate over the raw logs and unpacked data for TokenDescriptorChanged events raised by the NFPM contract.
type NFPMTokenDescriptorChangedIterator struct {
	Event *NFPMTokenDescriptorChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMTokenDescriptorChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMTokenDescriptorChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMTokenDescriptorChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMTokenDescriptorChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMTokenDescriptorChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMTokenDescriptorChanged represents a TokenDescriptorChanged event raised by the NFPM contract.
type NFPMTokenDescriptorChanged struct {
	TokenDescriptor common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterTokenDescriptorChanged is a free log retrieval operation binding the contract event 0x9a72f60932a1a6a1e1ceaa7a7dc51efcfe37685b729d8a680ab939f4612455a6.
//
// Solidity: event TokenDescriptorChanged(address indexed tokenDescriptor)
func (_NFPM *NFPMFilterer) FilterTokenDescriptorChanged(opts *bind.FilterOpts, tokenDescriptor []common.Address) (*NFPMTokenDescriptorChangedIterator, error) {

	var tokenDescriptorRule []interface{}
	for _, tokenDescriptorItem := range tokenDescriptor {
		tokenDescriptorRule = append(tokenDescriptorRule, tokenDescriptorItem)
	}

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "TokenDescriptorChanged", tokenDescriptorRule)
	if err != nil {
		return nil, err
	}
	return &NFPMTokenDescriptorChangedIterator{contract: _NFPM.contract, event: "TokenDescriptorChanged", logs: logs, sub: sub}, nil
}

// WatchTokenDescriptorChanged is a free log subscription operation binding the contract event 0x9a72f60932a1a6a1e1ceaa7a7dc51efcfe37685b729d8a680ab939f4612455a6.
//
// Solidity: event TokenDescriptorChanged(address indexed tokenDescriptor)
func (_NFPM *NFPMFilterer) WatchTokenDescriptorChanged(opts *bind.WatchOpts, sink chan<- *NFPMTokenDescriptorChanged, tokenDescriptor []common.Address) (event.Subscription, error) {

	var tokenDescriptorRule []interface{}
	for _, tokenDescriptorItem := range tokenDescriptor {
		tokenDescriptorRule = append(tokenDescriptorRule, tokenDescriptorItem)
	}

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "TokenDescriptorChanged", tokenDescriptorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMTokenDescriptorChanged)
				if err := _NFPM.contract.UnpackLog(event, "TokenDescriptorChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTokenDescriptorChanged is a log parse operation binding the contract event 0x9a72f60932a1a6a1e1ceaa7a7dc51efcfe37685b729d8a680ab939f4612455a6.
//
// Solidity: event TokenDescriptorChanged(address indexed tokenDescriptor)
func (_NFPM *NFPMFilterer) ParseTokenDescriptorChanged(log types.Log) (*NFPMTokenDescriptorChanged, error) {
	event := new(NFPMTokenDescriptorChanged)
	if err := _NFPM.contract.UnpackLog(event, "TokenDescriptorChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the NFPM contract.
type NFPMTransferIterator struct {
	Event *NFPMTransfer // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMTransfer)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMTransfer)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMTransfer represents a Transfer event raised by the NFPM contract.
type NFPMTransfer struct {
	From    common.Address
	To      common.Address
	TokenId *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_NFPM *NFPMFilterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address, tokenId []*big.Int) (*NFPMTransferIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &NFPMTransferIterator{contract: _NFPM.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_NFPM *NFPMFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *NFPMTransfer, from []common.Address, to []common.Address, tokenId []*big.Int) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMTransfer)
				if err := _NFPM.contract.UnpackLog(event, "Transfer", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTransfer is a log parse operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_NFPM *NFPMFilterer) ParseTransfer(log types.Log) (*NFPMTransfer, error) {
	event := new(NFPMTransfer)
	if err := _NFPM.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NFPMTransferOwnershipIterator is returned from FilterTransferOwnership and is used to iterate over the raw logs and unpacked data for TransferOwnership events raised by the NFPM contract.
type NFPMTransferOwnershipIterator struct {
	Event *NFPMTransferOwnership // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NFPMTransferOwnershipIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NFPMTransferOwnership)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NFPMTransferOwnership)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NFPMTransferOwnershipIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NFPMTransferOwnershipIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NFPMTransferOwnership represents a TransferOwnership event raised by the NFPM contract.
type NFPMTransferOwnership struct {
	Owner common.Address
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterTransferOwnership is a free log retrieval operation binding the contract event 0xcfaaa26691e16e66e73290fc725eee1a6b4e0e693a1640484937aac25ffb55a4.
//
// Solidity: event TransferOwnership(address indexed owner)
func (_NFPM *NFPMFilterer) FilterTransferOwnership(opts *bind.FilterOpts, owner []common.Address) (*NFPMTransferOwnershipIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _NFPM.contract.FilterLogs(opts, "TransferOwnership", ownerRule)
	if err != nil {
		return nil, err
	}
	return &NFPMTransferOwnershipIterator{contract: _NFPM.contract, event: "TransferOwnership", logs: logs, sub: sub}, nil
}

// WatchTransferOwnership is a free log subscription operation binding the contract event 0xcfaaa26691e16e66e73290fc725eee1a6b4e0e693a1640484937aac25ffb55a4.
//
// Solidity: event TransferOwnership(address indexed owner)
func (_NFPM *NFPMFilterer) WatchTransferOwnership(opts *bind.WatchOpts, sink chan<- *NFPMTransferOwnership, owner []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _NFPM.contract.WatchLogs(opts, "TransferOwnership", ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NFPMTransferOwnership)
				if err := _NFPM.contract.UnpackLog(event, "TransferOwnership", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTransferOwnership is a log parse operation binding the contract event 0xcfaaa26691e16e66e73290fc725eee1a6b4e0e693a1640484937aac25ffb55a4.
//
// Solidity: event TransferOwnership(address indexed owner)
func (_NFPM *NFPMFilterer) ParseTransferOwnership(log types.Log) (*NFPMTransferOwnership, error) {
	event := new(NFPMTransferOwnership)
	if err := _NFPM.contract.UnpackLog(event, "TransferOwnership", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

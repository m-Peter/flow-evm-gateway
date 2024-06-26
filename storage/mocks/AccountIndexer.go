// Code generated by mockery v2.21.4. DO NOT EDIT.

package mocks

import (
	big "math/big"

	common "github.com/onflow/go-ethereum/common"
	mock "github.com/stretchr/testify/mock"

	models "github.com/onflow/flow-evm-gateway/models"

	types "github.com/onflow/go-ethereum/core/types"
)

// AccountIndexer is an autogenerated mock type for the AccountIndexer type
type AccountIndexer struct {
	mock.Mock
}

// GetBalance provides a mock function with given fields: address
func (_m *AccountIndexer) GetBalance(address *common.Address) (*big.Int, error) {
	ret := _m.Called(address)

	var r0 *big.Int
	var r1 error
	if rf, ok := ret.Get(0).(func(*common.Address) (*big.Int, error)); ok {
		return rf(address)
	}
	if rf, ok := ret.Get(0).(func(*common.Address) *big.Int); ok {
		r0 = rf(address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*big.Int)
		}
	}

	if rf, ok := ret.Get(1).(func(*common.Address) error); ok {
		r1 = rf(address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNonce provides a mock function with given fields: address
func (_m *AccountIndexer) GetNonce(address *common.Address) (uint64, error) {
	ret := _m.Called(address)

	var r0 uint64
	var r1 error
	if rf, ok := ret.Get(0).(func(*common.Address) (uint64, error)); ok {
		return rf(address)
	}
	if rf, ok := ret.Get(0).(func(*common.Address) uint64); ok {
		r0 = rf(address)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func(*common.Address) error); ok {
		r1 = rf(address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Update provides a mock function with given fields: tx, receipt
func (_m *AccountIndexer) Update(tx models.Transaction, receipt *types.Receipt) error {
	ret := _m.Called(tx, receipt)

	var r0 error
	if rf, ok := ret.Get(0).(func(models.Transaction, *types.Receipt) error); ok {
		r0 = rf(tx, receipt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewAccountIndexer interface {
	mock.TestingT
	Cleanup(func())
}

// NewAccountIndexer creates a new instance of AccountIndexer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewAccountIndexer(t mockConstructorTestingTNewAccountIndexer) *AccountIndexer {
	mock := &AccountIndexer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

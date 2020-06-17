// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

import runner "github.com/open-integration/core/pkg/runner"

// Modem is an autogenerated mock type for the Modem type
type Modem struct {
	mock.Mock
}

// AddService provides a mock function with given fields: id, name, _a2
func (_m *Modem) AddService(id string, name string, _a2 runner.Runner) error {
	ret := _m.Called(id, name, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, runner.Runner) error); ok {
		r0 = rf(id, name, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Call provides a mock function with given fields: service, endpoint, arguments, fd
func (_m *Modem) Call(service string, endpoint string, arguments map[string]interface{}, fd string) ([]byte, error) {
	ret := _m.Called(service, endpoint, arguments, fd)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(string, string, map[string]interface{}, string) []byte); ok {
		r0 = rf(service, endpoint, arguments, fd)
	} else {
		r0 = ret.Get(0).([]byte)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, map[string]interface{}, string) error); ok {
		r1 = rf(service, endpoint, arguments, fd)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Destroy provides a mock function with given fields:
func (_m *Modem) Destroy() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Init provides a mock function with given fields:
func (_m *Modem) Init() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

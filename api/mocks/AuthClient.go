// Copyright 2021 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	dbus "github.com/northerntechhq/nt-connect/client/dbus"

	mock "github.com/stretchr/testify/mock"
)

// AuthClient is an autogenerated mock type for the AuthClient type
type AuthClient struct {
	mock.Mock
}

// Connect provides a mock function with given fields: objectName, objectPath, interfaceName
func (_m *AuthClient) Connect(objectName string, objectPath string, interfaceName string) error {
	ret := _m.Called(objectName, objectPath, interfaceName)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string) error); ok {
		r0 = rf(objectName, objectPath, interfaceName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// FetchJWTToken provides a mock function with given fields:
func (_m *AuthClient) FetchJWTToken() (bool, error) {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetJWTToken provides a mock function with given fields:
func (_m *AuthClient) GetJWTToken() (string, string, error) {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 string
	if rf, ok := ret.Get(1).(func() string); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(string)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetJwtTokenStateChangeChannel provides a mock function with given fields:
func (_m *AuthClient) GetJwtTokenStateChangeChannel() chan []dbus.SignalParams {
	ret := _m.Called()

	var r0 chan []dbus.SignalParams
	if rf, ok := ret.Get(0).(func() chan []dbus.SignalParams); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan []dbus.SignalParams)
		}
	}

	return r0
}

// WaitForJwtTokenStateChange provides a mock function with given fields:
func (_m *AuthClient) WaitForJwtTokenStateChange() ([]dbus.SignalParams, error) {
	ret := _m.Called()

	var r0 []dbus.SignalParams
	if rf, ok := ret.Get(0).(func() []dbus.SignalParams); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]dbus.SignalParams)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
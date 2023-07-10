// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/matyle/bililive-go/src/pkg/parser (interfaces: Parser)

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	url "net/url"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	live "github.com/matyle/bililive-go/src/live"
)

// MockParser is a mock of Parser interface.
type MockParser struct {
	ctrl     *gomock.Controller
	recorder *MockParserMockRecorder
}

// MockParserMockRecorder is the mock recorder for MockParser.
type MockParserMockRecorder struct {
	mock *MockParser
}

// NewMockParser creates a new mock instance.
func NewMockParser(ctrl *gomock.Controller) *MockParser {
	mock := &MockParser{ctrl: ctrl}
	mock.recorder = &MockParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockParser) EXPECT() *MockParserMockRecorder {
	return m.recorder
}

// ParseLiveStream mocks base method.
func (m *MockParser) ParseLiveStream(arg0 context.Context, arg1 *url.URL, arg2 live.Live, arg3 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseLiveStream", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// ParseLiveStream indicates an expected call of ParseLiveStream.
func (mr *MockParserMockRecorder) ParseLiveStream(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseLiveStream", reflect.TypeOf((*MockParser)(nil).ParseLiveStream), arg0, arg1, arg2, arg3)
}

// Stop mocks base method.
func (m *MockParser) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockParserMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockParser)(nil).Stop))
}

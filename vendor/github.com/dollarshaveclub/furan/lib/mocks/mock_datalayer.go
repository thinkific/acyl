// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/dollarshaveclub/furan/lib/datalayer (interfaces: DataLayer)

// Package mocks is a generated GoMock package.
package mocks

import (
	lib "github.com/dollarshaveclub/furan/generated/lib"
	gocql "github.com/gocql/gocql"
	gomock "github.com/golang/mock/gomock"
	ddtrace "gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	reflect "reflect"
)

// MockDataLayer is a mock of DataLayer interface
type MockDataLayer struct {
	ctrl     *gomock.Controller
	recorder *MockDataLayerMockRecorder
}

// MockDataLayerMockRecorder is the mock recorder for MockDataLayer
type MockDataLayerMockRecorder struct {
	mock *MockDataLayer
}

// NewMockDataLayer creates a new mock instance
func NewMockDataLayer(ctrl *gomock.Controller) *MockDataLayer {
	mock := &MockDataLayer{ctrl: ctrl}
	mock.recorder = &MockDataLayerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockDataLayer) EXPECT() *MockDataLayerMockRecorder {
	return m.recorder
}

// CreateBuild mocks base method
func (m *MockDataLayer) CreateBuild(arg0 ddtrace.Span, arg1 *lib.BuildRequest) (gocql.UUID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateBuild", arg0, arg1)
	ret0, _ := ret[0].(gocql.UUID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateBuild indicates an expected call of CreateBuild
func (mr *MockDataLayerMockRecorder) CreateBuild(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateBuild", reflect.TypeOf((*MockDataLayer)(nil).CreateBuild), arg0, arg1)
}

// DeleteBuild mocks base method
func (m *MockDataLayer) DeleteBuild(arg0 ddtrace.Span, arg1 gocql.UUID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteBuild", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteBuild indicates an expected call of DeleteBuild
func (mr *MockDataLayerMockRecorder) DeleteBuild(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteBuild", reflect.TypeOf((*MockDataLayer)(nil).DeleteBuild), arg0, arg1)
}

// GetBuildByID mocks base method
func (m *MockDataLayer) GetBuildByID(arg0 ddtrace.Span, arg1 gocql.UUID) (*lib.BuildStatusResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBuildByID", arg0, arg1)
	ret0, _ := ret[0].(*lib.BuildStatusResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBuildByID indicates an expected call of GetBuildByID
func (mr *MockDataLayerMockRecorder) GetBuildByID(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBuildByID", reflect.TypeOf((*MockDataLayer)(nil).GetBuildByID), arg0, arg1)
}

// GetBuildOutput mocks base method
func (m *MockDataLayer) GetBuildOutput(arg0 ddtrace.Span, arg1 gocql.UUID, arg2 string) ([]lib.BuildEvent, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBuildOutput", arg0, arg1, arg2)
	ret0, _ := ret[0].([]lib.BuildEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBuildOutput indicates an expected call of GetBuildOutput
func (mr *MockDataLayerMockRecorder) GetBuildOutput(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBuildOutput", reflect.TypeOf((*MockDataLayer)(nil).GetBuildOutput), arg0, arg1, arg2)
}

// SaveBuildOutput mocks base method
func (m *MockDataLayer) SaveBuildOutput(arg0 ddtrace.Span, arg1 gocql.UUID, arg2 []lib.BuildEvent, arg3 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveBuildOutput", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveBuildOutput indicates an expected call of SaveBuildOutput
func (mr *MockDataLayerMockRecorder) SaveBuildOutput(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveBuildOutput", reflect.TypeOf((*MockDataLayer)(nil).SaveBuildOutput), arg0, arg1, arg2, arg3)
}

// SetBuildCompletedTimestamp mocks base method
func (m *MockDataLayer) SetBuildCompletedTimestamp(arg0 ddtrace.Span, arg1 gocql.UUID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBuildCompletedTimestamp", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBuildCompletedTimestamp indicates an expected call of SetBuildCompletedTimestamp
func (mr *MockDataLayerMockRecorder) SetBuildCompletedTimestamp(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBuildCompletedTimestamp", reflect.TypeOf((*MockDataLayer)(nil).SetBuildCompletedTimestamp), arg0, arg1)
}

// SetBuildFlags mocks base method
func (m *MockDataLayer) SetBuildFlags(arg0 ddtrace.Span, arg1 gocql.UUID, arg2 map[string]bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBuildFlags", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBuildFlags indicates an expected call of SetBuildFlags
func (mr *MockDataLayerMockRecorder) SetBuildFlags(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBuildFlags", reflect.TypeOf((*MockDataLayer)(nil).SetBuildFlags), arg0, arg1, arg2)
}

// SetBuildState mocks base method
func (m *MockDataLayer) SetBuildState(arg0 ddtrace.Span, arg1 gocql.UUID, arg2 lib.BuildStatusResponse_BuildState) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBuildState", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBuildState indicates an expected call of SetBuildState
func (mr *MockDataLayerMockRecorder) SetBuildState(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBuildState", reflect.TypeOf((*MockDataLayer)(nil).SetBuildState), arg0, arg1, arg2)
}

// SetBuildTimeMetric mocks base method
func (m *MockDataLayer) SetBuildTimeMetric(arg0 ddtrace.Span, arg1 gocql.UUID, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBuildTimeMetric", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBuildTimeMetric indicates an expected call of SetBuildTimeMetric
func (mr *MockDataLayerMockRecorder) SetBuildTimeMetric(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBuildTimeMetric", reflect.TypeOf((*MockDataLayer)(nil).SetBuildTimeMetric), arg0, arg1, arg2)
}

// SetDockerImageSizesMetric mocks base method
func (m *MockDataLayer) SetDockerImageSizesMetric(arg0 ddtrace.Span, arg1 gocql.UUID, arg2, arg3 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetDockerImageSizesMetric", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetDockerImageSizesMetric indicates an expected call of SetDockerImageSizesMetric
func (mr *MockDataLayerMockRecorder) SetDockerImageSizesMetric(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetDockerImageSizesMetric", reflect.TypeOf((*MockDataLayer)(nil).SetDockerImageSizesMetric), arg0, arg1, arg2, arg3)
}

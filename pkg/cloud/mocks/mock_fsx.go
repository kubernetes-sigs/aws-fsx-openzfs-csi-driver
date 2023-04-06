// Code generated by MockGen. DO NOT EDIT.
// Source: sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/cloud (interfaces: FSx)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	request "github.com/aws/aws-sdk-go/aws/request"
	fsx "github.com/aws/aws-sdk-go/service/fsx"
	gomock "github.com/golang/mock/gomock"
)

// MockFSx is a mock of FSx interface.
type MockFSx struct {
	ctrl     *gomock.Controller
	recorder *MockFSxMockRecorder
}

// MockFSxMockRecorder is the mock recorder for MockFSx.
type MockFSxMockRecorder struct {
	mock *MockFSx
}

// NewMockFSx creates a new mock instance.
func NewMockFSx(ctrl *gomock.Controller) *MockFSx {
	mock := &MockFSx{ctrl: ctrl}
	mock.recorder = &MockFSxMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFSx) EXPECT() *MockFSxMockRecorder {
	return m.recorder
}

// CreateSnapshotWithContext mocks base method.
func (m *MockFSx) CreateSnapshotWithContext(arg0 context.Context, arg1 *fsx.CreateSnapshotInput, arg2 ...request.Option) (*fsx.CreateSnapshotOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CreateSnapshotWithContext", varargs...)
	ret0, _ := ret[0].(*fsx.CreateSnapshotOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateSnapshotWithContext indicates an expected call of CreateSnapshotWithContext.
func (mr *MockFSxMockRecorder) CreateSnapshotWithContext(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateSnapshotWithContext", reflect.TypeOf((*MockFSx)(nil).CreateSnapshotWithContext), varargs...)
}

// DeleteSnapshotWithContext mocks base method.
func (m *MockFSx) DeleteSnapshotWithContext(arg0 context.Context, arg1 *fsx.DeleteSnapshotInput, arg2 ...request.Option) (*fsx.DeleteSnapshotOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteSnapshotWithContext", varargs...)
	ret0, _ := ret[0].(*fsx.DeleteSnapshotOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteSnapshotWithContext indicates an expected call of DeleteSnapshotWithContext.
func (mr *MockFSxMockRecorder) DeleteSnapshotWithContext(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteSnapshotWithContext", reflect.TypeOf((*MockFSx)(nil).DeleteSnapshotWithContext), varargs...)
}

// DescribeFileSystemsWithContext mocks base method.
func (m *MockFSx) DescribeFileSystemsWithContext(arg0 context.Context, arg1 *fsx.DescribeFileSystemsInput, arg2 ...request.Option) (*fsx.DescribeFileSystemsOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DescribeFileSystemsWithContext", varargs...)
	ret0, _ := ret[0].(*fsx.DescribeFileSystemsOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeFileSystemsWithContext indicates an expected call of DescribeFileSystemsWithContext.
func (mr *MockFSxMockRecorder) DescribeFileSystemsWithContext(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeFileSystemsWithContext", reflect.TypeOf((*MockFSx)(nil).DescribeFileSystemsWithContext), varargs...)
}

// DescribeSnapshotsWithContext mocks base method.
func (m *MockFSx) DescribeSnapshotsWithContext(arg0 context.Context, arg1 *fsx.DescribeSnapshotsInput, arg2 ...request.Option) (*fsx.DescribeSnapshotsOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DescribeSnapshotsWithContext", varargs...)
	ret0, _ := ret[0].(*fsx.DescribeSnapshotsOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeSnapshotsWithContext indicates an expected call of DescribeSnapshotsWithContext.
func (mr *MockFSxMockRecorder) DescribeSnapshotsWithContext(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeSnapshotsWithContext", reflect.TypeOf((*MockFSx)(nil).DescribeSnapshotsWithContext), varargs...)
}

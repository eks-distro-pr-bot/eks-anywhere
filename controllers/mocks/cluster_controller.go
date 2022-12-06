// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/aws/eks-anywhere/controllers (interfaces: AWSIamConfigReconciler)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	v1alpha1 "github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	controller "github.com/aws/eks-anywhere/pkg/controller"
	logr "github.com/go-logr/logr"
	gomock "github.com/golang/mock/gomock"
)

// MockAWSIamConfigReconciler is a mock of AWSIamConfigReconciler interface.
type MockAWSIamConfigReconciler struct {
	ctrl     *gomock.Controller
	recorder *MockAWSIamConfigReconcilerMockRecorder
}

// MockAWSIamConfigReconcilerMockRecorder is the mock recorder for MockAWSIamConfigReconciler.
type MockAWSIamConfigReconcilerMockRecorder struct {
	mock *MockAWSIamConfigReconciler
}

// NewMockAWSIamConfigReconciler creates a new mock instance.
func NewMockAWSIamConfigReconciler(ctrl *gomock.Controller) *MockAWSIamConfigReconciler {
	mock := &MockAWSIamConfigReconciler{ctrl: ctrl}
	mock.recorder = &MockAWSIamConfigReconcilerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAWSIamConfigReconciler) EXPECT() *MockAWSIamConfigReconcilerMockRecorder {
	return m.recorder
}

// EnsureCASecret mocks base method.
func (m *MockAWSIamConfigReconciler) EnsureCASecret(arg0 context.Context, arg1 logr.Logger, arg2 *v1alpha1.Cluster) (controller.Result, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsureCASecret", arg0, arg1, arg2)
	ret0, _ := ret[0].(controller.Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EnsureCASecret indicates an expected call of EnsureCASecret.
func (mr *MockAWSIamConfigReconcilerMockRecorder) EnsureCASecret(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsureCASecret", reflect.TypeOf((*MockAWSIamConfigReconciler)(nil).EnsureCASecret), arg0, arg1, arg2)
}

// Reconcile mocks base method.
func (m *MockAWSIamConfigReconciler) Reconcile(arg0 context.Context, arg1 logr.Logger, arg2 *v1alpha1.Cluster) (controller.Result, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Reconcile", arg0, arg1, arg2)
	ret0, _ := ret[0].(controller.Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Reconcile indicates an expected call of Reconcile.
func (mr *MockAWSIamConfigReconcilerMockRecorder) Reconcile(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Reconcile", reflect.TypeOf((*MockAWSIamConfigReconciler)(nil).Reconcile), arg0, arg1, arg2)
}

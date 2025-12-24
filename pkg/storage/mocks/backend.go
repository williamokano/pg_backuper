// Code generated manually. DO NOT EDIT.

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/williamokano/pg_backuper/pkg/storage"
)

// MockBackend is a mock implementation of the storage.Backend interface
type MockBackend struct {
	mock.Mock
}

// Name provides a mock function with given fields:
func (m *MockBackend) Name() string {
	ret := m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Type provides a mock function with given fields:
func (m *MockBackend) Type() string {
	ret := m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Write provides a mock function with given fields: ctx, sourcePath, destPath
func (m *MockBackend) Write(ctx context.Context, sourcePath string, destPath string) error {
	ret := m.Called(ctx, sourcePath, destPath)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, sourcePath, destPath)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Delete provides a mock function with given fields: ctx, path
func (m *MockBackend) Delete(ctx context.Context, path string) error {
	ret := m.Called(ctx, path)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, path)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// List provides a mock function with given fields: ctx, pattern
func (m *MockBackend) List(ctx context.Context, pattern string) ([]storage.FileInfo, error) {
	ret := m.Called(ctx, pattern)

	var r0 []storage.FileInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]storage.FileInfo, error)); ok {
		return rf(ctx, pattern)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []storage.FileInfo); ok {
		r0 = rf(ctx, pattern)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]storage.FileInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, pattern)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Stat provides a mock function with given fields: ctx, path
func (m *MockBackend) Stat(ctx context.Context, path string) (*storage.FileInfo, error) {
	ret := m.Called(ctx, path)

	var r0 *storage.FileInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*storage.FileInfo, error)); ok {
		return rf(ctx, path)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *storage.FileInfo); ok {
		r0 = rf(ctx, path)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*storage.FileInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Exists provides a mock function with given fields: ctx, path
func (m *MockBackend) Exists(ctx context.Context, path string) (bool, error) {
	ret := m.Called(ctx, path)

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (bool, error)); ok {
		return rf(ctx, path)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) bool); ok {
		r0 = rf(ctx, path)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Close provides a mock function with given fields:
func (m *MockBackend) Close() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewMockBackend creates a new instance of MockBackend
func NewMockBackend(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockBackend {
	mock_1 := &MockBackend{}
	mock_1.Mock.Test(t)

	t.Cleanup(func() { mock_1.AssertExpectations(t) })

	return mock_1
}

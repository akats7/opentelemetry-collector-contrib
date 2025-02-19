// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errConsumeType = errors.New("Error in consume type assertion")

type PSConstants struct {
	RetryInterval time.Duration
	RetryGap      time.Duration
	MaxRetries    int
}

type TryLock struct {
	lock sync.Mutex
}

func (t *TryLock) TryExecute(fn func(int), arg int) {
	if t.lock.TryLock() {
		defer t.lock.Unlock()
		fn(arg)
	}
}

func NewTryLock() *TryLock {
	return &TryLock{}
}

type CancelManager struct {
	cancelFunc context.CancelFunc
}

func (c *CancelManager) Cancel() {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
}

func (c *CancelManager) UpdateFn(cancelFunc context.CancelFunc) {
	c.cancelFunc = cancelFunc
}

// Manages cancel function for retry goroutine, ends up cleaner than using channels
type RetryState struct {
	lock        sync.Mutex
	cancelRetry context.CancelFunc
}

func (m *RetryState) UpdateCancelFunc(newCancelFunc context.CancelFunc) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.cancelRetry = newCancelFunc
}

func (m *RetryState) InvokeCancel() {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.cancelRetry != nil {
		m.cancelRetry()
	}
}

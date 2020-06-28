package manager

import (
	"sync"

	"github.com/Hatch1fy/errors"
	"github.com/hatchify/queue"
)

// New will implement a new instance of transactions Manager
func New() *Manager {
	var m Manager
	return &m
}

// Manager manages multiple trasnactions
type Manager struct {
	mux sync.Mutex

	q *queue.Queue

	out chan error
	ins []chan error
}

// Run will call the provided run func from within the collection of transactions
func (m *Manager) Run(run func() error, fns ...TxnFn) (err error) {
	// Acquire mutex lock
	m.mux.Lock()
	// Defer the release of mutex lock
	defer m.mux.Unlock()
	// Defer teardown of manager
	defer m.teardown()

	// Set queue
	m.q = queue.New(len(fns), 0)
	// Initialize out channel
	m.out = make(chan error, len(fns))
	// Initialize inbound channel slice
	m.ins = make([]chan error, 0, len(fns))

	// Open provided transaction functions
	start, end := m.openTxns(fns)

	// Wait for all transactions to start
	start.Wait()

	// Now that transactions have been initialized, call target function
	err = run()

	// Push error to inbound channels
	m.pushErrorToInbounds(err)

	// Wait for all transactions to finish
	end.Wait()

	// Get errors outbound out channel
	errs := m.newErrorsFromOutbound()

	if err != nil {
		// We encountered an error when calling our target function. There is no need to collect
		// additional errors, return
		return
	}

	// Collect and combine any errors we encountered during the transaction close
	return errs.Err()
}

func (m *Manager) pushErrorToInbounds(err error) {
	// Push error to all open transactions
	for _, in := range m.ins {
		// Push error to inbound channel
		in <- err
		// Close inbound channel
		close(in)
	}
}

func (m *Manager) newErrorsFromOutbound() (errs errors.ErrorList) {
	// Even if we've already encountered an error. We want to allow the channel to clear to avoid
	// any potential memory leaks.
	for txnErr := range m.out {
		errs.Push(txnErr)
	}

	return
}

func (m *Manager) openTxns(fns []TxnFn) (start, end sync.WaitGroup) {
	// Set waitgroups
	start.Add(len(fns))
	end.Add(len(fns))

	// Iterate through transaction functions
	for _, fn := range fns {
		// Create inbound channel by opening transaction
		inCh := m.openTxn(fn, m.out, &start, &end)
		// Append inbound channel to inbound transactions slice
		m.ins = append(m.ins, inCh)
	}

	return
}

func (m *Manager) openTxn(fn TxnFn, out chan error, start, end *sync.WaitGroup) (in chan error) {
	in = make(chan error, 1)
	m.q.New(func() {
		fn(start, in, out)
		end.Done()
	})

	return
}

func (m *Manager) teardown() {
	// Close queue
	m.q.Close()
	// Set queue reference to nil
	m.q = nil
	// Close outbound channel
	close(m.out)
}

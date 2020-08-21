package manager

import (
	"sync"

	"github.com/hatchify/errors"
	"github.com/hatchify/queue"
)

// New will implement a new instance of transactions Manager
func New(fns ...TxnFn) *Manager {
	var m Manager
	m.fns = fns
	return &m
}

// Manager manages multiple trasnactions
type Manager struct {
	mux sync.Mutex

	fns []TxnFn

	q *queue.Queue

	out chan error
	ins []chan error
}

// Run will call the provided run func from within the collection of transactions
func (m *Manager) Run(run func() error) (err error) {
	// Acquire mutex lock
	m.mux.Lock()
	// Defer the release of mutex lock
	defer m.mux.Unlock()
	// Call internal run func
	return m.run(run)
}

func (m *Manager) initRun() {
	// Set queue
	m.q = queue.New(len(m.fns), 0)
	// Initialize out channel
	m.out = make(chan error, len(m.fns))
	// Initialize inbound channel slice
	m.ins = make([]chan error, 0, len(m.fns))
}

func (m *Manager) run(run func() error) (err error) {
	// Defer teardown of manager
	defer m.teardown()

	// Initialize items needed for run
	m.initRun()

	// Open provided transaction functions
	done := m.openTxns()

	// Now that transactions have been initialized, call target function
	err = run()

	// Push error to inbound channels
	m.pushErrorToInbounds(err)

	// Wait for all transactions to finish
	done.Wait()

	// Since all transactions have ended, we can safely close outbound channel
	close(m.out)

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

func (m *Manager) openTxns() (done *sync.WaitGroup) {
	var start, end sync.WaitGroup
	// Set waitgroups
	start.Add(len(m.fns))
	end.Add(len(m.fns))

	// Iterate through transaction functions
	for _, fn := range m.fns {
		// Create inbound channel by opening transaction
		inCh := m.openTxn(fn, m.out, &start, &end)
		// Append inbound channel to inbound transactions slice
		m.ins = append(m.ins, inCh)
	}

	// Wait for all transactions to start
	start.Wait()

	// Assign reference to done
	done = &end
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

	// Set references to nil
	m.q = nil
	m.ins = nil
	m.out = nil
}

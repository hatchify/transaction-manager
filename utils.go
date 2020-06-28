package manager

import "sync"

// TxnFn is called by the transaction Manager
type TxnFn func(started *sync.WaitGroup, inboundC <-chan error, outboundC chan<- error)

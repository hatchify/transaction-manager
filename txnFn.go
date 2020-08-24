package manager

import (
	"context"
	"sync"
)

// TxnFn is called by the transaction Manager
type TxnFn func(ctx context.Context, started *sync.WaitGroup, inboundC <-chan error, outboundC chan<- error)

// package extcontext provides extensions for contexts

package extcontext

import (
	"context"
	"os"
	"os/signal"
	"time"
)

// WithSignals returns a copy of parent with a new Done channel. The returned
// context's Done channel is closed when the returned cancel function is
// called, when the parent context's Done channel is closed, or when the
// process receives one of the given signals, whichever happens first.
//
// Note that calling this function without providing any signals will listen to
// all incoming signals to this process. See os/signal for more details.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete.
func WithSignals(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	child, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	go func() {
		select {
		case <-child.Done():
		case <-ch:
			cancel()
		}
	}()
	return child, cancel
}

// WithGracePeriod returns a copy of parent with a new Done channel. The
// returned context's Done channel is closed when the returned cancel function
// is called or, if the parent's context's Done channel is closed first, after
// the expiration of the provided grace period.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this Context complete.
func WithGracePeriod(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	child, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		select {
		case <-child.Done():
			return
		case <-parent.Done():
		}
		select {
		case <-time.After(d):
		case <-child.Done():
		}
	}()

	return graceContext{
		parent: parent,
		child:  child,

		gracePeriod: d,
	}, cancel
}

type graceContext struct {
	parent, child context.Context

	gracePeriod time.Duration
}

func (g graceContext) Deadline() (deadline time.Time, ok bool) {
	deadline, ok = g.parent.Deadline()
	if ok {
		deadline.Add(g.gracePeriod)
	}
	return
}

func (g graceContext) Done() <-chan struct{} { return g.child.Done() }
func (g graceContext) Err() error            { return g.child.Err() }

func (g graceContext) Value(key interface{}) interface{} { return g.parent.Value(key) }

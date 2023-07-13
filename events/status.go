package events

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	log "k8s.io/klog/v2"
)

// A Watcher watches event updates.
type Watcher struct {
	ctx         context.Context
	errCh       chan error
	wstop       func()
	cancel      func()
	eventStates map[types.UID]string
	ch          chan *EventStatus
	stdout      io.Writer
	warningf    func(string, ...any)

	mu               sync.Mutex
	progress         bool
	currentNamespace string
	currentEvent     types.UID
}

// NewWatcher returns a Watcher on the provided client or an error.  The cancel
// function is called when the Watcher determines an event has permanently
// failed.  The Watcher will exit if the context provided is canceled, an error
// is encountered, or Cleanup is called.
func NewWatcher(ctx context.Context, client kubernetes.Interface, cancel func()) (*Watcher, error) {
	ch, stop, err := WatchEventStatus(ctx, client, "")
	if err != nil {
		return nil, err
	}
	w := newWatcher(ctx, cancel, ch, stop)
	go w.watch()
	return w, nil
}

func newWatcher(ctx context.Context, cancel func(), ch chan *EventStatus, stop func()) *Watcher {
	w := &Watcher{
		ctx:         ctx,
		ch:          ch,
		wstop:       stop,
		cancel:      cancel,
		stdout:      os.Stdout,
		eventStates: map[types.UID]string{},
		warningf:    log.Warningf,
	}
	// A channel is used to record errors from the watcher to prevent any
	// possible race conditions if Cleanup is called while an update is
	// happening.  At most one error will be written to the channel.
	w.errCh = make(chan error, 1)
	w.display("Displaying state changes for events")
	return w
}

// SetProgress determins if progress output should be displayed while watching.
func (w *Watcher) SetProgress(value bool) {
	w.mu.Lock()
	w.progress = value
	w.mu.Unlock()
}

func (w *Watcher) stop() {
	w.mu.Lock()
	stop := w.wstop
	w.wstop = nil
	w.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// Cleanup should be called when the Watcher is no longer needed.  If the
// Watcher encountered an error the provided err is logged and the Watcher error
// is returned, otherwise err is returned.
func (w *Watcher) Cleanup(err error) error {
	w.stop()
	select {
	case werr := <-w.errCh:
		if err != nil {
			w.warningf("Deploy() failed: %v", err)
		}
		w.warningf("Deployment failed: %v", werr)
		return werr
	default:
	}
	return err
}

func (w *Watcher) watch() {
	defer w.stop()
	for {
		select {
		case s, ok := <-w.ch:
			if !ok || !w.updateEvent(s) {
				return
			}
		case <-w.ctx.Done():
			return
		}
	}
}

var timeNow = func() string { return time.Now().Format("15:04:05 ") }

func (w *Watcher) display(format string, v ...any) {
	if w.progress {
		fmt.Fprintf(w.stdout, timeNow()+format+"\n", v...)
	}
}

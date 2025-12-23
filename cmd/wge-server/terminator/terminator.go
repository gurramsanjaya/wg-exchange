package terminator

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var term *terminator

// These should block and wait for ctx.cancel(), can also trigger cancel internally
type BlockCtxFn func(ctx context.Context, cancel context.CancelFunc)

type terminator struct {
	sync.Mutex
	sigChan  chan os.Signal
	ctx      context.Context
	cancel   context.CancelFunc
	blockFns []BlockCtxFn
	numSub   int
}

func init() {
	term = &terminator{
		sigChan:  make(chan os.Signal, 2),
		ctx:      nil,
		blockFns: make([]BlockCtxFn, 0, 2),
	}
}

func triggerBlockCtxFn(block BlockCtxFn) {
	block(term.ctx, term.cancel)

	term.Lock()
	defer term.Unlock()
	term.numSub -= 1
}

func monitor() {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case sig := <-term.sigChan:
			// signal received
			log.Println("signal received:", sig, "stopping gracefully.")
			term.cancel()
			return
		case <-term.ctx.Done():
			// ctx cancelled
			return
		default:
			<-tick.C
		}
	}
}

func HookInto(fn BlockCtxFn) error {
	term.Lock()
	defer term.Unlock()
	if fn == nil {
		return errors.New("block fn can't be nil")
	}
	term.blockFns = append(term.blockFns, fn)
	term.numSub += 1
	return nil
}

// call in main
func StartTerminator(wait int32) {
	signal.Notify(term.sigChan, syscall.SIGTERM, syscall.SIGINT)
	if wait == 0 {
		term.ctx, term.cancel = context.WithCancel(context.Background())
	} else {
		term.ctx, term.cancel = context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	}
	for i := 0; i < len(term.blockFns); i++ {
		go triggerBlockCtxFn(term.blockFns[i])
	}

	// blocking
	monitor()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-term.sigChan:
			log.Println("stopping anyway...")
			ticker.Stop()
			os.Exit(1)
		default:
		}

		term.Lock()
		if term.numSub <= 0 {
			term.Unlock()
			log.Println("all subscribers stopped")
			return
		}
		term.Unlock()
		<-ticker.C
	}
}

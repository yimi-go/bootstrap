package bootstrap

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/exp/slog"
	"golang.org/x/sync/errgroup"

	"github.com/yimi-go/runner"
	"github.com/yimi-go/shutdown"
	"github.com/yimi-go/shutdown/posixsignal"
)

type Bootstrap interface {
	Run(ctx context.Context) error
}

type bootstrap struct {
	beforeRun func(ctx context.Context) error
	onRun     func(ctx context.Context) error
	runners   []runner.Runner
	gs        shutdown.Controller
}

func (b bootstrap) Run(ctx context.Context) error {
	logger := slog.Ctx(ctx)
	if len(b.runners) == 0 {
		logger.Log(slog.ErrorLevel, "no runners, abort.")
		return nil
	}
	before := b.beforeRun
	if before != nil {
		if err := before(ctx); err != nil {
			return err
		}
	}
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return b.gs.Wait(egCtx)
	})
	waitStart := &sync.WaitGroup{}
	for _, r := range b.runners {
		r := r
		b.gs.AddShutdownCallback(shutdown.CallbackFunc(func(ctx context.Context, event shutdown.Event) error {
			if logger.Enabled(slog.InfoLevel) {
				logger.Info(fmt.Sprintf("Stopping runner: %s, cause: %s", r.Name(), event.Reason()))
			}
			err := r.Stop(ctx)
			if err != nil {
				return errors.WithMessagef(err, "stopping %s failed", r.Name())
			}
			if logger.Enabled(slog.InfoLevel) {
				logger.Info(fmt.Sprintf("Runner stoped: %s", r.Name()))
			}
			return nil
		}))
		waitStart.Add(1)
		eg.Go(func() error {
			if logger.Enabled(slog.InfoLevel) {
				logger.Info(fmt.Sprintf("Starting runner: %s", r.Name()))
			}
			waitStart.Done()
			err := r.Run(egCtx)
			if err != nil {
				return errors.WithMessagef(err, "starting %s failed", r.Name())
			}
			return nil
		})
	}
	waitStart.Wait()
	if logger.Enabled(slog.InfoLevel) {
		logger.Info("bootstrap started.")
	}
	eg.Go(func() error {
		fn := b.onRun
		if fn != nil {
			err := fn(egCtx)
			if err != nil {
				return errors.WithMessagef(err, "onRun err")
			}
		}
		return nil
	})
	err := eg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return errors.WithMessagef(err, "bootstrap run err")
	}
	return nil
}

func New(opts ...Option) Bootstrap {
	b := bootstrap{
		gs: shutdown.NewGraceful(
			shutdown.WithTimeout(time.Second),
			shutdown.WithErrorHandler(shutdown.ErrorHandleFunc(func(ctx context.Context, err error) {
				slog.Ctx(ctx).Error("error when shutting down", err)
			})),
			shutdown.WithTrigger(posixsignal.NewTrigger()),
		),
	}
	for _, opt := range opts {
		opt(&b)
	}
	return b
}

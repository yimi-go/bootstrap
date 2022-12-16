package bootstrap

import (
	"context"

	"github.com/yimi-go/runner"
	"github.com/yimi-go/shutdown"
)

type Option func(b *bootstrap)

func WithShutdown(gs shutdown.Controller) Option {
	return func(b *bootstrap) {
		if gs == nil {
			return
		}
		b.gs = gs
	}
}

func WithBeforeRun(before func(ctx context.Context) error) Option {
	return func(b *bootstrap) {
		b.beforeRun = before
	}
}

func WithOnRun(fn func(ctx context.Context) error) Option {
	return func(b *bootstrap) {
		b.onRun = fn
	}
}

func WithRunners(rs ...runner.Runner) Option {
	return func(b *bootstrap) {
		b.runners = append(b.runners, rs...)
	}
}

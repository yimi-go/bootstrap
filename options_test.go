package bootstrap

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestWithShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	c := NewMockController(ctrl)
	b := bootstrap{}
	WithShutdown(c)(&b)
	assert.Same(t, c, b.gs)
	WithShutdown(nil)(&b)
	assert.Same(t, c, b.gs)
}

func TestWithBeforeRun(t *testing.T) {
	count := 0
	b := bootstrap{}
	fn := func(ctx context.Context) error {
		count++
		return nil
	}
	WithBeforeRun(fn)(&b)
	assert.NotNil(t, b.beforeRun)
	assert.Nil(t, b.beforeRun(context.Background()))
	assert.Equal(t, 1, count)
}

func TestWithOnRun(t *testing.T) {
	count := 0
	b := bootstrap{}
	fn := func(ctx context.Context) error {
		count++
		return nil
	}
	WithOnRun(fn)(&b)
	assert.NotNil(t, b.onRun)
	assert.Nil(t, b.onRun(context.Background()))
	assert.Equal(t, 1, count)
}

func TestWithRunners(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	b := bootstrap{}
	WithRunners(NewMockRunner(ctrl), NewMockRunner(ctrl))(&b)
	assert.Len(t, b.runners, 2)
}

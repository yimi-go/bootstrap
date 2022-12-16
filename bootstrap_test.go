package bootstrap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slog"
)

func TestNew(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		b := New()
		assert.NotNil(t, b)
		assert.IsType(t, bootstrap{}, b)
		assert.NotNil(t, b.(bootstrap).gs)
	})
	t.Run("opts", func(t *testing.T) {
		count := 0
		opt := Option(func(b *bootstrap) {
			count++
		})
		b := New(opt)
		assert.NotNil(t, b)
		assert.Equal(t, 1, count)
	})
}

func bufLogCtx(ctx context.Context, buf *bytes.Buffer) context.Context {
	return slog.NewContext(ctx, slog.New(slog.NewJSONHandler(buf)).WithContext(ctx))
}

func printAndJson(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Logf("output: %s", buf.String())
	var maps []map[string]any
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		if errors.Is(scanner.Err(), io.EOF) {
			continue
		}
		mp := map[string]any{}
		if err := json.Unmarshal(scanner.Bytes(), &mp); err == nil {
			maps = append(maps, mp)
		} else {
			t.Logf("err line: %v: %s", err, scanner.Text())
		}
	}
	buf.Reset()
	return maps
}

func TestBootstrap_Run(t *testing.T) {
	t.Run("no_runner", func(t *testing.T) {
		logBuf := &bytes.Buffer{}
		ctx := context.Background()
		ctx = bufLogCtx(ctx, logBuf)
		b := New()
		err := b.Run(ctx)
		assert.Nil(t, err)
		mps := printAndJson(t, logBuf)
		assert.Len(t, mps, 1)
		assert.Equal(t, "ERROR", mps[0][slog.LevelKey])
	})
	t.Run("run", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		logBuf := &bytes.Buffer{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = bufLogCtx(ctx, logBuf)
		r := NewMockRunner(ctrl)
		r.EXPECT().Name().Return("testRunner").MinTimes(1)
		r.EXPECT().Run(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
		stopped := make(chan struct{}, 1)
		r.EXPECT().Stop(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			stopped <- struct{}{}
			return nil
		})
		beforeCount := 0
		onRunCount := 0
		b := New(WithRunners(r), WithBeforeRun(func(ctx context.Context) error {
			beforeCount++
			return nil
		}), WithOnRun(func(ctx context.Context) error {
			onRunCount++
			return nil
		}))
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Run(ctx)
			assert.Nil(t, err)
		}()
		go func() {
			<-time.After(time.Millisecond * 10)
			cancel()
		}()
		wg.Wait()
		<-stopped
		assert.Equal(t, 1, beforeCount)
		assert.Equal(t, 1, onRunCount)
		mps := printAndJson(t, logBuf)
		assert.Len(t, mps, 4)
		assert.Equal(t, slog.InfoLevel.String(), mps[0][slog.LevelKey])
		assert.Contains(t, mps[0][slog.MessageKey], "Starting runner: ")
	})
	t.Run("before_fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		logBuf := &bytes.Buffer{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = bufLogCtx(ctx, logBuf)
		r := NewMockRunner(ctrl)
		r.EXPECT().Name().Return("testRunner").Times(0)
		r.EXPECT().Run(gomock.Any()).Times(0)
		r.EXPECT().Stop(gomock.Any()).Times(0)
		beforeCount := 0
		b := New(WithRunners(r), WithBeforeRun(func(ctx context.Context) error {
			beforeCount++
			return errors.New("test")
		}))
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Run(ctx)
			assert.NotNil(t, err)
			t.Logf("%v", err)
		}()
		wg.Wait()
		assert.Equal(t, 1, beforeCount)
		mps := printAndJson(t, logBuf)
		assert.Empty(t, mps)
	})
	t.Run("onRun_fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		logBuf := &bytes.Buffer{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = bufLogCtx(ctx, logBuf)
		r := NewMockRunner(ctrl)
		r.EXPECT().Name().Return("testRunner").MinTimes(1)
		r.EXPECT().Run(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
		stopped := make(chan struct{}, 1)
		r.EXPECT().Stop(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			stopped <- struct{}{}
			return nil
		})
		onRunCount := 0
		b := New(WithRunners(r), WithOnRun(func(ctx context.Context) error {
			onRunCount++
			return errors.New("test")
		}))
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Run(ctx)
			assert.NotNil(t, err)
			t.Logf("%v", err)
		}()
		wg.Wait()
		<-stopped
		assert.Equal(t, 1, onRunCount)
		mps := printAndJson(t, logBuf)
		assert.Len(t, mps, 4)
		assert.Equal(t, slog.InfoLevel.String(), mps[0][slog.LevelKey])
		assert.Contains(t, mps[0][slog.MessageKey], "Starting runner: ")
	})
	t.Run("runner_stop_fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		logBuf := &bytes.Buffer{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = bufLogCtx(ctx, logBuf)
		r := NewMockRunner(ctrl)
		r.EXPECT().Name().Return("testRunner").MinTimes(1)
		r.EXPECT().Run(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
		stopped := make(chan struct{}, 1)
		r.EXPECT().Stop(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			stopped <- struct{}{}
			return errors.New("test")
		})
		b := New(WithRunners(r))
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Run(ctx)
			assert.Nil(t, err)
		}()
		go func() {
			<-time.After(time.Millisecond * 10)
			cancel()
		}()
		wg.Wait()
		<-stopped
		mps := printAndJson(t, logBuf)
		assert.Len(t, mps, 4)
		assert.Equal(t, slog.InfoLevel.String(), mps[0][slog.LevelKey])
		assert.Contains(t, mps[0][slog.MessageKey], "Starting runner: ")
	})
	t.Run("run_err", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		logBuf := &bytes.Buffer{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = bufLogCtx(ctx, logBuf)
		r := NewMockRunner(ctrl)
		r.EXPECT().Name().Return("testRunner").MinTimes(1)
		r.EXPECT().Run(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			return errors.New("test")
		})
		stopped := make(chan struct{}, 1)
		r.EXPECT().Stop(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			stopped <- struct{}{}
			return nil
		})
		beforeCount := 0
		onRunCount := 0
		b := New(WithRunners(r), WithBeforeRun(func(ctx context.Context) error {
			beforeCount++
			return nil
		}), WithOnRun(func(ctx context.Context) error {
			onRunCount++
			return nil
		}))
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Run(ctx)
			assert.NotNil(t, err)
			t.Logf("%v", err)
		}()
		wg.Wait()
		<-stopped
		assert.Equal(t, 1, beforeCount)
		assert.Equal(t, 1, onRunCount)
		mps := printAndJson(t, logBuf)
		assert.Len(t, mps, 4)
		assert.Equal(t, slog.InfoLevel.String(), mps[0][slog.LevelKey])
		assert.Contains(t, mps[0][slog.MessageKey], "Starting runner: ")
	})
}

package workerpool

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"hh-parser/pkg/logger"
)

type Task func(ctx context.Context) error

type Pool struct {
	workers  int
	tasks    chan Task
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	errMu    sync.Mutex
	errs     []error
	errLimit int
	closed   atomic.Bool  // закрыт ли канал задач
	stopped  atomic.Bool  // остановлен ли пул
	active   atomic.Int32 // количество активных задач
}

func NewPool(workers int, errLimit int) *Pool {
	if errLimit <= 0 {
		errLimit = 100
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		workers:  workers,
		tasks:    make(chan Task, workers*100),
		ctx:      ctx,
		cancel:   cancel,
		errs:     make([]error, 0, errLimit),
		errLimit: errLimit,
	}
}

func (p *Pool) Start() {
	logger.Log.Debug("Starting worker pool", "workers", p.workers)
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *Pool) AddTask(task Task) bool {
	if p.stopped.Load() || p.closed.Load() {
		logger.Log.Warn("Attempted to add task to stopped pool")
		return false
	}

	select {
	case p.tasks <- task:
		return true
	case <-p.ctx.Done():
		logger.Log.Warn("Task not added (context cancelled)")
		return false
	}
}

// Stop останавливает пул с принудительным завершением активных задач
func (p *Pool) Stop() {
	if p.stopped.Swap(true) {
		return
	}

	logger.Log.Info("Stopping worker pool...")

	// Закрываем канал для новых задач
	p.closed.Store(true)
	close(p.tasks)

	// Ждем завершения всех задач
	p.wg.Wait()

	// Отменяем контекст для гарантии
	p.cancel()

	logger.Log.Info("Worker pool stopped", "total_errors", len(p.errs))
}

// Shutdown gracefully останавливает пул с таймаутом
func (p *Pool) Shutdown(ctx context.Context) error {
	if p.stopped.Swap(true) {
		return nil
	}

	logger.Log.Info("Gracefully shutting down worker pool...")

	// Закрываем канал для новых задач
	p.closed.Store(true)
	close(p.tasks)

	// Канал для ожидания завершения
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// Ожидаем завершения или таймаута
	select {
	case <-done:
		logger.Log.Info("All workers finished gracefully")
		p.cancel()
		return nil
	case <-ctx.Done():
		logger.Log.Warn("Shutdown timeout, forcing cancellation")
		p.cancel()
		return fmt.Errorf("shutdown timeout: %w", ctx.Err())
	}
}

func (p *Pool) Errors() []error {
	p.errMu.Lock()
	defer p.errMu.Unlock()
	return append([]error{}, p.errs...)
}

func (p *Pool) ErrorCount() int {
	p.errMu.Lock()
	defer p.errMu.Unlock()
	return len(p.errs)
}

func (p *Pool) ActiveTasks() int {
	return int(p.active.Load())
}

func (p *Pool) worker(id int) {
	defer p.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Error("Worker panicked",
				"worker_id", id,
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()

	logger.Log.Debug("Worker started", "worker_id", id)

	for {
		select {
		case task, ok := <-p.tasks:
			if !ok {
				logger.Log.Debug("Worker stopping (tasks channel closed)", "worker_id", id)
				return
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Log.Error("Task panicked",
							"worker_id", id,
							"panic", r)
						p.errMu.Lock()
						err := fmt.Errorf("task panicked: %v", r)
						if len(p.errs) < p.errLimit {
							p.errs = append(p.errs, err)
						}
						p.errMu.Unlock()
					}
				}()

				p.active.Add(1)
				err := task(p.ctx)
				p.active.Add(-1)

				if err != nil {
					p.errMu.Lock()
					if len(p.errs) < p.errLimit {
						p.errs = append(p.errs, err)
					} else if p.errLimit > 0 {
						p.errs[p.errLimit-1] = err
					}
					p.errMu.Unlock()
					logger.Log.Warn("Task failed", "worker_id", id, "error", err)
				}
			}()

		case <-p.ctx.Done():
			logger.Log.Debug("Worker stopping (context cancelled)", "worker_id", id)
			return
		}
	}
}

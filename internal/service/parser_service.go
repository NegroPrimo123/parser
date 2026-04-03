package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"hh-parser/internal/interfaces"
	"hh-parser/internal/models"
	"hh-parser/internal/parser"
	"hh-parser/pkg/limiter"
	"hh-parser/pkg/logger"
	"hh-parser/pkg/metrics"
	"hh-parser/pkg/retry"
	"hh-parser/pkg/workerpool"
)

type ParseResult struct {
	TotalFound  int
	ParsedCount int
	Errors      []error
}

type ParserService struct {
	parser         interfaces.VacancyParser
	storage        interfaces.VacancyStorage
	exporter       interfaces.VacancyExporter
	searchLimiter  *limiter.RateLimiter
	detailsLimiter *limiter.RateLimiter
	pool           *workerpool.Pool
	retryConfig    *retry.Config
}

func NewParserService(
	parser interfaces.VacancyParser,
	storage interfaces.VacancyStorage,
	exporter interfaces.VacancyExporter,
	workers int,
) *ParserService {
	return &ParserService{
		parser:         parser,
		storage:        storage,
		exporter:       exporter,
		searchLimiter:  limiter.NewRateLimiter(1),
		detailsLimiter: limiter.NewRateLimiter(2),
		pool:           workerpool.NewPool(workers, 100),
		retryConfig:    retry.DefaultConfig(),
	}
}

func (s *ParserService) Run(ctx context.Context, query string, pages, perPage int) (*ParseResult, error) {
	s.pool.Start()
	defer s.pool.Stop()

	var totalVacancies int32
	var parsedCount int32
	var errsMu sync.Mutex
	var allErrors []error

	errorsChan := make(chan error, pages)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for err := range errorsChan {
			errsMu.Lock()
			allErrors = append(allErrors, err)
			errsMu.Unlock()
		}
	}()

	for page := 0; page < pages; page++ {
		select {
		case <-ctx.Done():
			close(errorsChan)
			wg.Wait()
			return nil, ctx.Err()
		default:
		}

		if err := s.searchLimiter.Wait(ctx); err != nil {
			close(errorsChan)
			wg.Wait()
			return nil, fmt.Errorf("rate limiter cancelled: %w", err)
		}

		// Используем интерфейс
		searchRespInterface, err := s.parser.SearchVacancies(ctx, query, page, perPage)
		if err != nil {
			errorsChan <- fmt.Errorf("page %d: %w", page, err)
			continue
		}

		// Приводим к нужному типу
		searchResp, ok := searchRespInterface.(*parser.HHSearchResponse)
		if !ok {
			errorsChan <- fmt.Errorf("page %d: invalid response type", page)
			continue
		}

		if searchResp == nil || len(searchResp.Items) == 0 {
			continue
		}

		atomic.AddInt32(&totalVacancies, int32(len(searchResp.Items)))

		for _, item := range searchResp.Items {
			vacancyItem := item
			if !s.pool.AddTask(func(taskCtx context.Context) error {
				if err := s.detailsLimiter.Wait(taskCtx); err != nil {
					return err
				}

				var details *models.Vacancy
				err := retry.Do(taskCtx, s.retryConfig, func() error {
					var retryErr error
					details, retryErr = s.parser.GetVacancyDetails(taskCtx, vacancyItem.ID)
					return retryErr
				})

				if err != nil {
					metrics.VacanciesProcessed.WithLabelValues("error").Inc()
					return fmt.Errorf("failed to get details for %s: %w", vacancyItem.ID, err)
				}

				if vacancyItem.Salary != nil {
					details.SalaryFrom = vacancyItem.Salary.From
					details.SalaryTo = vacancyItem.Salary.To
					details.Currency = vacancyItem.Salary.Currency
				}
				details.Employer = vacancyItem.Employer.Name
				details.City = vacancyItem.Area.Name

				err = retry.Do(taskCtx, s.retryConfig, func() error {
					return s.storage.SaveVacancy(taskCtx, details)
				})

				if err != nil {
					metrics.VacanciesProcessed.WithLabelValues("error").Inc()
					return fmt.Errorf("failed to save %s: %w", details.HHID, err)
				}

				if s.exporter != nil {
					if err := s.exporter.ExportVacancy(details); err != nil {
						logger.Log.Warn("Failed to export to CSV", "vacancy_id", details.HHID, "error", err)
					}
				}

				atomic.AddInt32(&parsedCount, 1)
				metrics.VacanciesProcessed.WithLabelValues("success").Inc()

				logger.Log.Debug("Vacancy processed", "id", details.HHID, "name", details.Name)
				return nil
			}) {
				errorsChan <- fmt.Errorf("failed to add task for vacancy %s: pool stopped", vacancyItem.ID)
			}
		}
	}

	close(errorsChan)
	wg.Wait()

	poolErrors := s.pool.Errors()
	errsMu.Lock()
	allErrors = append(allErrors, poolErrors...)
	errsMu.Unlock()

	return &ParseResult{
		TotalFound:  int(atomic.LoadInt32(&totalVacancies)),
		ParsedCount: int(atomic.LoadInt32(&parsedCount)),
		Errors:      allErrors,
	}, nil
}

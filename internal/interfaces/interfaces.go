package interfaces

import (
	"context"
	"hh-parser/internal/models"
)

type VacancyParser interface {
	SearchVacancies(ctx context.Context, query string, page, perPage int) (interface{}, error)
	GetVacancyDetails(ctx context.Context, id string) (*models.Vacancy, error)
}

type VacancyStorage interface {
	SaveVacancy(ctx context.Context, vacancy *models.Vacancy) error
	GetAllVacancies(ctx context.Context) ([]models.Vacancy, error)
	Close() error
}

type VacancyExporter interface {
	ExportVacancy(vacancy *models.Vacancy) error
	GetFilePath() string
	Close() error
}

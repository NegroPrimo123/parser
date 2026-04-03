package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"hh-parser/internal/models"

	_ "github.com/lib/pq"
)

type Storage struct {
	db *sql.DB
}

func NewPostgresStorage(host, port, user, password, dbname string) (*Storage, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable client_encoding=UTF8",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Установка кодировки клиента
	_, err = db.Exec("SET client_encoding = 'UTF8'")
	if err != nil {
		return nil, err
	}

	return &Storage{db: db}, nil
}

func (s *Storage) SaveVacancy(ctx context.Context, vacancy *models.Vacancy) error {
	// Очистка строк от некорректных символов
	cleanString := func(s string) string {
		// Замена проблемных символов
		s = strings.ReplaceAll(s, "\x00", "") // Удаление null байтов
		s = strings.ToValidUTF8(s, "�")       // Замена невалидных UTF-8 символов
		return s
	}

	query := `
        INSERT INTO vacancies (hh_id, name, salary_from, salary_to, currency, employer, city, requirement, responsibility, url, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        ON CONFLICT (hh_id) DO UPDATE SET
            name = EXCLUDED.name,
            salary_from = EXCLUDED.salary_from,
            salary_to = EXCLUDED.salary_to,
            employer = EXCLUDED.employer,
            city = EXCLUDED.city,
            requirement = EXCLUDED.requirement,
            responsibility = EXCLUDED.responsibility,
            updated_at = NOW()
    `

	_, err := s.db.ExecContext(ctx, query,
		vacancy.HHID,
		cleanString(vacancy.Name),
		vacancy.SalaryFrom,
		vacancy.SalaryTo,
		vacancy.Currency,
		cleanString(vacancy.Employer),
		cleanString(vacancy.City),
		cleanString(vacancy.Requirement),
		cleanString(vacancy.Responsibility),
		vacancy.URL,
		vacancy.CreatedAt)

	return err
}

func (s *Storage) GetAllVacancies(ctx context.Context) ([]models.Vacancy, error) {
	query := `
		SELECT id, hh_id, name, salary_from, salary_to, currency, 
		       employer, city, requirement, responsibility, url, created_at
		FROM vacancies 
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vacancies []models.Vacancy
	for rows.Next() {
		var v models.Vacancy
		err := rows.Scan(
			&v.ID, &v.HHID, &v.Name,
			&v.SalaryFrom, &v.SalaryTo, &v.Currency,
			&v.Employer, &v.City, &v.Requirement,
			&v.Responsibility, &v.URL, &v.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		vacancies = append(vacancies, v)
	}

	return vacancies, rows.Err()
}

func (s *Storage) DB() *sql.DB {
	return s.db
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) GetVacanciesPaginated(ctx context.Context, limit, offset int) ([]models.Vacancy, error) {
	query := `
		SELECT id, hh_id, name, salary_from, salary_to, currency, 
		       employer, city, requirement, responsibility, url, created_at
		FROM vacancies 
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vacancies []models.Vacancy
	for rows.Next() {
		var v models.Vacancy
		err := rows.Scan(
			&v.ID, &v.HHID, &v.Name,
			&v.SalaryFrom, &v.SalaryTo, &v.Currency,
			&v.Employer, &v.City, &v.Requirement,
			&v.Responsibility, &v.URL, &v.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		vacancies = append(vacancies, v)
	}

	return vacancies, rows.Err()
}

func (s *Storage) GetVacanciesCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM vacancies").Scan(&count)
	return count, err
}

func (s *Storage) SaveVacanciesBatch(ctx context.Context, vacancies []*models.Vacancy) error {
	if len(vacancies) == 0 {
		return nil
	}

	// Подготовка запроса
	query := `
		INSERT INTO vacancies (hh_id, name, salary_from, salary_to, currency, employer, city, requirement, responsibility, url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (hh_id) DO UPDATE SET
			name = EXCLUDED.name,
			salary_from = EXCLUDED.salary_from,
			salary_to = EXCLUDED.salary_to,
			employer = EXCLUDED.employer,
			city = EXCLUDED.city,
			requirement = EXCLUDED.requirement,
			responsibility = EXCLUDED.responsibility,
			updated_at = NOW()
	`

	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Выполняем batch insert в транзакции
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, vacancy := range vacancies {
		cleanString := func(s string) string {
			s = strings.ReplaceAll(s, "\x00", "")
			s = strings.ToValidUTF8(s, "�")
			if len(s) > 10000 {
				s = s[:10000]
			}
			return s
		}

		_, err := tx.Stmt(stmt).ExecContext(ctx,
			vacancy.HHID,
			cleanString(vacancy.Name),
			vacancy.SalaryFrom,
			vacancy.SalaryTo,
			vacancy.Currency,
			cleanString(vacancy.Employer),
			cleanString(vacancy.City),
			cleanString(vacancy.Requirement),
			cleanString(vacancy.Responsibility),
			vacancy.URL,
			vacancy.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Storage) validateAndClean(vacancy *models.Vacancy) {
	cleanString := func(s string, maxLen int) string {
		s = strings.ReplaceAll(s, "\x00", "")
		s = strings.ToValidUTF8(s, "�")
		if len(s) > maxLen {
			s = s[:maxLen]
		}
		return s
	}
	
	vacancy.Name = cleanString(vacancy.Name, 500)
	vacancy.Employer = cleanString(vacancy.Employer, 255)
	vacancy.City = cleanString(vacancy.City, 255)
	vacancy.Requirement = cleanString(vacancy.Requirement, 10000)
	vacancy.Responsibility = cleanString(vacancy.Responsibility, 10000)
}
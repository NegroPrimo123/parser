package api

import (
	"encoding/csv"
	"net/http"
	"strconv"

	"hh-parser/internal/storage"
	"hh-parser/pkg/logger"
)

func ExportCSVHandler(storage *storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Получаем общее количество
		total, err := storage.GetVacanciesCount(r.Context())
		if err != nil {
			logger.Log.Error("Failed to get vacancies count", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=vacancies.csv")

		// Создаем CSV writer
		writer := csv.NewWriter(w)
		writer.Comma = ';'
		defer writer.Flush()

		// Заголовки
		headers := []string{
			"ID", "HH ID", "Название", "Зарплата от", "Зарплата до", "Валюта",
			"Работодатель", "Город", "Требования", "URL", "Дата создания",
		}
		if err := writer.Write(headers); err != nil {
			logger.Log.Error("Failed to write CSV headers", "error", err)
			return
		}

		// Пагинация - по 1000 записей за раз
		limit := 1000
		offset := 0

		for offset < total {
			select {
			case <-r.Context().Done():
				logger.Log.Warn("CSV export cancelled by client")
				return
			default:
			}

			vacancies, err := storage.GetVacanciesPaginated(r.Context(), limit, offset)
			if err != nil {
				logger.Log.Error("Failed to get vacancies batch", "error", err, "offset", offset)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Записываем данные
			for _, v := range vacancies {
				salaryFrom := ""
				if v.SalaryFrom != nil {
					salaryFrom = strconv.Itoa(*v.SalaryFrom)
				}
				salaryTo := ""
				if v.SalaryTo != nil {
					salaryTo = strconv.Itoa(*v.SalaryTo)
				}

				record := []string{
					strconv.Itoa(v.ID),
					v.HHID,
					v.Name,
					salaryFrom,
					salaryTo,
					v.Currency,
					v.Employer,
					v.City,
					v.Requirement,
					v.URL,
					v.CreatedAt.Format("2006-01-02 15:04:05"),
				}
				if err := writer.Write(record); err != nil {
					logger.Log.Error("Failed to write CSV record", "error", err)
					continue
				}
			}

			writer.Flush() // Периодически сбрасываем буфер
			offset += limit

			logger.Log.Debug("CSV export progress", "processed", offset, "total", total)
		}

		logger.Log.Info("CSV export completed", "count", total)
	}
}

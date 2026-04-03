package exporter

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"hh-parser/internal/models"
)

type CSVExporter struct {
	file   *os.File
	writer *csv.Writer
}

func NewCSVExporter(filename string) (*CSVExporter, error) {
	// Создаем директорию для результатов, если её нет
	if err := os.MkdirAll("results", 0755); err != nil {
		return nil, err
	}

	// Генерируем имя файла с датой
	if filename == "" {
		filename = fmt.Sprintf("results/vacancies_%s.csv", time.Now().Format("20060102_150405"))
	}

	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	writer := csv.NewWriter(file)
	writer.Comma = ';' // Используем точку с запятой для Excel

	// Записываем заголовки
	headers := []string{
		"ID", "HH ID", "Название", "Зарплата от", "Зарплата до", "Валюта",
		"Работодатель", "Город", "Требования", "URL", "Дата создания",
	}

	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	writer.Flush()

	return &CSVExporter{
		file:   file,
		writer: writer,
	}, nil
}

func (e *CSVExporter) ExportVacancy(vacancy *models.Vacancy) error {
	salaryFrom := ""
	if vacancy.SalaryFrom != nil {
		salaryFrom = fmt.Sprintf("%d", *vacancy.SalaryFrom)
	}

	salaryTo := ""
	if vacancy.SalaryTo != nil {
		salaryTo = fmt.Sprintf("%d", *vacancy.SalaryTo)
	}

	// Обрезаем длинные тексты для CSV
	requirement := vacancy.Requirement
	if len(requirement) > 500 {
		requirement = requirement[:500] + "..."
	}

	record := []string{
		fmt.Sprintf("%d", vacancy.ID),
		vacancy.HHID,
		vacancy.Name,
		salaryFrom,
		salaryTo,
		vacancy.Currency,
		vacancy.Employer,
		vacancy.City,
		requirement,
		vacancy.URL,
		vacancy.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	if err := e.writer.Write(record); err != nil {
		return err
	}

	return nil
}

func (e *CSVExporter) GetFilePath() string {
	return e.file.Name()
}

func (e *CSVExporter) Close() error {
	e.writer.Flush()
	return e.file.Close()
}

package models

import "time"

type Vacancy struct {
	ID             int       `json:"id"`
	HHID           string    `json:"hh_id" db:"hh_id"`
	Name           string    `json:"name"`
	SalaryFrom     *int      `json:"salary_from" db:"salary_from"`
	SalaryTo       *int      `json:"salary_to" db:"salary_to"`
	Currency       string    `json:"currency"`
	Employer       string    `json:"employer"`
	City           string    `json:"city"`
	Requirement    string    `json:"requirement"`
	Responsibility string    `json:"responsibility"`
	URL            string    `json:"url"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

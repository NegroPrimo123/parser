CREATE TABLE IF NOT EXISTS vacancies (
    id SERIAL PRIMARY KEY,
    hh_id VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(500) NOT NULL,
    salary_from INTEGER,
    salary_to INTEGER,
    currency VARCHAR(3),
    employer VARCHAR(255),
    city VARCHAR(255),
    requirement TEXT,
    responsibility TEXT,
    url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_hh_id ON vacancies(hh_id);
CREATE INDEX idx_created_at ON vacancies(created_at);
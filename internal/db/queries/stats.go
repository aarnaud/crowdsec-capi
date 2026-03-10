package queries

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MachineStats struct {
	Total     int `json:"total"`
	Validated int `json:"validated"`
	Pending   int `json:"pending"`
	Blocked   int `json:"blocked"`
}

type DecisionStats struct {
	Total    int            `json:"total"`
	ByOrigin map[string]int `json:"by_origin"`
	ByType   map[string]int `json:"by_type"`
}

type CountryCount struct {
	Country string `json:"country"`
	Count   int    `json:"count"`
}

type Stats struct {
	Machines         MachineStats   `json:"machines"`
	Decisions        DecisionStats  `json:"decisions"`
	SignalsLast24h   int            `json:"signals_last_24h"`
	SignalsByCountry []CountryCount `json:"signals_by_country"`
}

func GetStats(ctx context.Context, db *pgxpool.Pool) (*Stats, error) {
	s := &Stats{
		Decisions: DecisionStats{
			ByOrigin: make(map[string]int),
			ByType:   make(map[string]int),
		},
	}

	// Machine counts
	rows, err := db.Query(ctx, `SELECT status, COUNT(*) FROM machines GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		s.Machines.Total += count
		switch status {
		case "validated":
			s.Machines.Validated = count
		case "pending":
			s.Machines.Pending = count
		case "blocked":
			s.Machines.Blocked = count
		}
	}
	rows.Close()

	// Decision counts by origin and type
	rows2, err := db.Query(ctx, `
		SELECT origin, type, COUNT(*)
		FROM decisions
		WHERE is_deleted = FALSE AND expires_at > NOW()
		GROUP BY origin, type
	`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var origin, dtype string
		var count int
		if err := rows2.Scan(&origin, &dtype, &count); err != nil {
			return nil, err
		}
		s.Decisions.Total += count
		s.Decisions.ByOrigin[origin] += count
		s.Decisions.ByType[dtype] += count
	}
	rows2.Close()

	// Signals last 24h
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM signals WHERE created_at > NOW() - INTERVAL '24 hours'
	`).Scan(&s.SignalsLast24h); err != nil {
		return nil, err
	}

	// Signals by country (last 30 days, top 100)
	rows3, err := db.Query(ctx, `
		SELECT source_country, COUNT(*) AS cnt
		FROM signals
		WHERE source_country IS NOT NULL AND source_country != ''
		  AND created_at > NOW() - INTERVAL '30 days'
		GROUP BY source_country
		ORDER BY cnt DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows3.Close()
	for rows3.Next() {
		var cc CountryCount
		if err := rows3.Scan(&cc.Country, &cc.Count); err != nil {
			return nil, err
		}
		s.SignalsByCountry = append(s.SignalsByCountry, cc)
	}
	if s.SignalsByCountry == nil {
		s.SignalsByCountry = []CountryCount{}
	}

	return s, rows3.Err()
}

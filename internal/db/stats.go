package db

import (
	"fmt"
	"time"
)

type StatEntry struct {
	What string `json:"what"`
	Hits int64  `json:"hits"`
	Node string `json:"node"`
}

type TimeSeriesPoint struct {
	Bucket string `json:"bucket"`
	Allow  int64  `json:"allow"`
	Deny   int64  `json:"deny"`
	Total  int64  `json:"total"`
}

func (d *Database) UpsertStat(table, what, node string, hits int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	allowed := map[string]bool{"hosts": true, "procs": true, "addrs": true, "ports": true, "users": true}
	if !allowed[table] {
		return nil
	}

	_, err := d.db.Exec(
		"INSERT INTO "+table+" (what, hits, node) VALUES (?, ?, ?) ON CONFLICT(what, node) DO UPDATE SET hits=excluded.hits",
		what, hits, node,
	)
	return err
}

func (d *Database) GetStats(table string, limit int) ([]StatEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	allowed := map[string]bool{"hosts": true, "procs": true, "addrs": true, "ports": true, "users": true}
	if !allowed[table] {
		return nil, nil
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := d.db.Query("SELECT what, hits, node FROM "+table+" ORDER BY hits DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []StatEntry
	for rows.Next() {
		var e StatEntry
		if err := rows.Scan(&e.What, &e.Hits, &e.Node); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (d *Database) GetConnectionTimeSeries(hours, bucketMinutes int, node string) ([]TimeSeriesPoint, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if bucketMinutes <= 0 {
		bucketMinutes = 15
	}

	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:04:05")

	query := fmt.Sprintf(`
		SELECT
			strftime('%%Y-%%m-%%d %%H:', time) || printf('%%02d', (CAST(strftime('%%M', time) AS INTEGER) / %d) * %d) AS bucket,
			SUM(CASE WHEN action = 'allow' THEN 1 ELSE 0 END) AS allow_count,
			SUM(CASE WHEN action IN ('deny','reject') THEN 1 ELSE 0 END) AS deny_count,
			COUNT(*) AS total
		FROM connections
		WHERE time >= ?`, bucketMinutes, bucketMinutes)

	args := []interface{}{since}
	if node != "" {
		query += " AND node = ?"
		args = append(args, node)
	}

	query += " GROUP BY bucket ORDER BY bucket ASC"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Bucket, &p.Allow, &p.Deny, &p.Total); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

type TopIPEntry struct {
	IP   string `json:"ip"`
	Hits int64  `json:"hits"`
}

func (d *Database) GetTopDestinationIPs(limit, hours int) ([]TopIPEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:04:05")

	rows, err := d.db.Query(`
		SELECT dst_ip, COUNT(*) AS hits
		FROM connections
		WHERE time >= ? AND dst_ip != ''
		GROUP BY dst_ip
		ORDER BY hits DESC
		LIMIT ?`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []TopIPEntry
	for rows.Next() {
		var e TopIPEntry
		if err := rows.Scan(&e.IP, &e.Hits); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (d *Database) GetTopBlocked(dimension string, limit, hours int) ([]StatEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:04:05")

	var col string
	switch dimension {
	case "hosts":
		col = "dst_host"
	case "processes":
		col = "process"
	default:
		col = "dst_host"
	}

	query := fmt.Sprintf(`
		SELECT %s AS what, COUNT(*) AS hits, '' AS node
		FROM connections
		WHERE action IN ('deny','reject') AND time >= ? AND %s != ''
		GROUP BY %s
		ORDER BY hits DESC
		LIMIT ?`, col, col, col)

	rows, err := d.db.Query(query, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []StatEntry
	for rows.Next() {
		var e StatEntry
		if err := rows.Scan(&e.What, &e.Hits, &e.Node); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

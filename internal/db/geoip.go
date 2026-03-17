package db

import "time"

type GeoIPEntry struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

func (d *Database) GetGeoIP(ip string) (*GeoIPEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var e GeoIPEntry
	err := d.db.QueryRow(
		"SELECT ip, country, country_code, city, lat, lon FROM geoip_cache WHERE ip = ?", ip,
	).Scan(&e.IP, &e.Country, &e.CountryCode, &e.City, &e.Lat, &e.Lon)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (d *Database) GetGeoIPBatch(ips []string) (map[string]*GeoIPEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(ips) == 0 {
		return nil, nil
	}

	// Build IN clause with placeholders
	placeholders := make([]byte, 0, len(ips)*2)
	args := make([]interface{}, len(ips))
	for i, ip := range ips {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args[i] = ip
	}

	rows, err := d.db.Query(
		"SELECT ip, country, country_code, city, lat, lon FROM geoip_cache WHERE ip IN ("+string(placeholders)+") AND cached_at >= datetime('now', '-7 days')",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*GeoIPEntry)
	for rows.Next() {
		var e GeoIPEntry
		if err := rows.Scan(&e.IP, &e.Country, &e.CountryCode, &e.City, &e.Lat, &e.Lon); err != nil {
			return nil, err
		}
		result[e.IP] = &e
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *Database) UpsertGeoIP(e *GeoIPEntry) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		`INSERT INTO geoip_cache (ip, country, country_code, city, lat, lon, cached_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(ip) DO UPDATE SET country=excluded.country, country_code=excluded.country_code,
		   city=excluded.city, lat=excluded.lat, lon=excluded.lon, cached_at=excluded.cached_at`,
		e.IP, e.Country, e.CountryCode, e.City, e.Lat, e.Lon,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

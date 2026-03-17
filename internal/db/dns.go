package db

import (
	"fmt"
	"strings"
)

type DNSDomain struct {
	ID        int64  `json:"id"`
	Node      string `json:"node"`
	Domain    string `json:"domain"`
	IP        string `json:"ip"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	HitCount  int64  `json:"hit_count"`
}

type DNSDomainFilter struct {
	Node   string
	Domain string
	IP     string
	Search string
	Limit  int
	Offset int
}

type DNSServerSummary struct {
	Node      string `json:"node"`
	DstIP     string `json:"dst_ip"`
	Process   string `json:"process"`
	Protocol  string `json:"protocol"`
	Hits      int64  `json:"hits"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	Action    string `json:"action"`
}

func (d *Database) UpsertDNSDomain(node, domain, ip, timestamp string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO dns_domains (node, domain, ip, first_seen, last_seen, hit_count)
		VALUES (?, ?, ?, ?, ?, 1)
		ON CONFLICT(node, domain, ip) DO UPDATE SET
			last_seen = ?,
			hit_count = hit_count + 1`,
		node, domain, ip, timestamp, timestamp, timestamp,
	)
	return err
}

func (d *Database) GetDNSDomains(f *DNSDomainFilter) ([]DNSDomain, int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	where := []string{"1=1"}
	args := []any{}

	if f.Node != "" {
		where = append(where, "node = ?")
		args = append(args, f.Node)
	}
	if f.Domain != "" {
		where = append(where, "domain LIKE ?")
		args = append(args, "%"+f.Domain+"%")
	}
	if f.IP != "" {
		where = append(where, "ip = ?")
		args = append(args, f.IP)
	}
	if f.Search != "" {
		where = append(where, "(domain LIKE ? OR ip LIKE ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := d.db.QueryRow("SELECT COUNT(*) FROM dns_domains WHERE "+whereClause, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := max(f.Offset, 0)

	query := fmt.Sprintf(
		"SELECT id, node, domain, ip, first_seen, last_seen, hit_count FROM dns_domains WHERE %s ORDER BY hit_count DESC, last_seen DESC LIMIT ? OFFSET ?",
		whereClause,
	)
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var domains []DNSDomain
	for rows.Next() {
		var dom DNSDomain
		if err := rows.Scan(&dom.ID, &dom.Node, &dom.Domain, &dom.IP, &dom.FirstSeen, &dom.LastSeen, &dom.HitCount); err != nil {
			return nil, 0, err
		}
		domains = append(domains, dom)
	}
	return domains, total, rows.Err()
}

func (d *Database) GetDNSServerSummary(node string, limit, offset int) ([]DNSServerSummary, int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	where := []string{"dst_port = 53"}
	args := []any{}

	if node != "" {
		where = append(where, "node = ?")
		args = append(args, node)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := d.db.QueryRow(
		"SELECT COUNT(*) FROM (SELECT 1 FROM connections WHERE "+whereClause+" GROUP BY node, dst_ip, process, protocol)",
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	offset = max(offset, 0)

	query := fmt.Sprintf(`
		SELECT
			node,
			dst_ip,
			process,
			protocol,
			COUNT(*) AS hits,
			MIN(time) AS first_seen,
			MAX(time) AS last_seen,
			COALESCE(MAX(action), '') AS action
		FROM connections
		WHERE %s
		GROUP BY node, dst_ip, process, protocol
		ORDER BY hits DESC, last_seen DESC
		LIMIT ? OFFSET ?
	`, whereClause)
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var summaries []DNSServerSummary
	for rows.Next() {
		var s DNSServerSummary
		if err := rows.Scan(&s.Node, &s.DstIP, &s.Process, &s.Protocol, &s.Hits, &s.FirstSeen, &s.LastSeen, &s.Action); err != nil {
			return nil, 0, err
		}
		summaries = append(summaries, s)
	}
	return summaries, total, rows.Err()
}

func (d *Database) PurgeDNSDomains() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM dns_domains")
	return err
}

// DeleteDNSAllowRules removes all dns-allow-* rules for a given node,
// used to clean up stale allow rules before re-applying a new set.
func (d *Database) DeleteDNSAllowRules(node string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM rules WHERE node = ? AND name LIKE 'dns-allow-%'", node)
	return err
}

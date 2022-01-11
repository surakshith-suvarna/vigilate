package dbrepo

import (
	"context"
	"log"
	"time"

	"github.com/tsawler/vigilate/internal/models"
)

func (m *postgresDBRepo) InsertHost(h models.Host) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `INSERT INTO hosts(host_name, canonical_name, url, ip, ipv6, location, os, active, created_at, updated_at)
			Values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) returning id`

	var newID int

	err := m.DB.QueryRowContext(ctx, query,
		h.HostName,
		h.CanonicalName,
		h.URL,
		h.IP,
		h.IPV6,
		h.Location,
		h.OS,
		h.Active,
		time.Now(),
		time.Now()).Scan(&newID)

	if err != nil {
		return newID, err
	}

	stmt := `Insert into host_services (host_id, service_id, active, schedule_number, schedule_unit,
			status, created_at, updated_at) VALUES($1, 1, 0, 3, 'm', 'pending', $2, $3)`

	_, err = m.DB.ExecContext(ctx, stmt, newID, time.Now(), time.Now())
	if err != nil {
		log.Println(err)
		return newID, err
		//In prod create delete by newid
	}
	return newID, nil
}

func (m *postgresDBRepo) GetHostById(id int) (models.Host, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var h models.Host

	query := `SELECT id, host_name, canonical_name, url, ip, ipv6, location, os, active, created_at, updated_at
			from hosts where id = $1`

	row := m.DB.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&h.ID,
		&h.HostName,
		&h.CanonicalName,
		&h.URL,
		&h.IP,
		&h.IPV6,
		&h.Location,
		&h.OS,
		&h.Active,
		&h.CreatedAt,
		&h.UpdatedAt,
	)
	if err != nil {
		return h, err
	}
	return h, nil
}

func (m *postgresDBRepo) UpdateHost(h models.Host) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stmt := `UPDATE hosts set host_name=$1, canonical_name=$2, url=$3, ip=$4, ipv6=$5, location=$6,
			os=$7, active=$8, updated_at=$9 where id=$10`

	_, err := m.DB.ExecContext(ctx, stmt,
		h.HostName,
		h.CanonicalName,
		h.URL,
		h.IP,
		h.IPV6,
		h.Location,
		h.OS,
		h.Active,
		time.Now(),
		h.ID)
	if err != nil {
		return err
	}
	return nil
}

func (m *postgresDBRepo) AllHosts() ([]models.Host, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var hosts []models.Host

	query := `SELECT id, host_name, canonical_name, url, ip, ipv6, location, os, active, created_at, updated_at
			from hosts order by host_name`

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		log.Println(err)
		return hosts, err
	}
	defer rows.Close()

	for rows.Next() {
		var host models.Host
		err = rows.Scan(
			&host.ID,
			&host.HostName,
			&host.CanonicalName,
			&host.URL,
			&host.IP,
			&host.IPV6,
			&host.Location,
			&host.OS,
			&host.Active,
			&host.CreatedAt,
			&host.UpdatedAt,
		)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		hosts = append(hosts, host)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
		return hosts, err
	}
	return hosts, nil
}

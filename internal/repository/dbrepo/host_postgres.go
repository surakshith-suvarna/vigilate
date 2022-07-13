package dbrepo

import (
	"context"
	"database/sql"
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
		//In prod create delete by newId
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

	query = `SELECT 
				hs.id, hs.host_id, hs.service_id, hs.schedule_number, hs.schedule_unit, 
				hs.last_check, hs.active, hs.status, hs.created_at, hs.updated_at, 
				s.id, s.service_name, s.active, s.icon, s.created_at, s.updated_at, hs.last_message
			FROM 
				host_services hs
				Left Join services s on (s.id = hs.service_id)
			Where 
				hs.host_id = $1	`

	rows, err := m.DB.QueryContext(ctx, query, h.ID)
	if err != nil {
		return h, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)
	var hostServices []models.HostServices
	for rows.Next() {
		var hs models.HostServices
		err = rows.Scan(
			&hs.ID,
			&hs.HostID,
			&hs.ServiceID,
			&hs.SecheduleNumber,
			&hs.ScheduleUnit,
			&hs.LastCheck,
			&hs.Active,
			&hs.Status,
			&hs.CreatedAt,
			&hs.UpdatedAt,
			&hs.Service.ID,
			&hs.Service.ServiceName,
			&hs.Service.Active,
			&hs.Service.Icon,
			&hs.Service.CreatedAt,
			&hs.Service.UpdatedAt,
			&hs.LastMessage,
		)
		if err != nil {
			log.Println(err)
			return h, err
		}
		hostServices = append(hostServices, hs)
	}
	if err = rows.Err(); err != nil {
		return h, err
	}
	h.HostServices = hostServices
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
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

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

		serviceQuery := `SELECT
							hs.id, hs.host_id, hs.service_id, hs.schedule_number, hs.schedule_unit, 
							hs.last_check, hs.active, hs.status, hs.created_at, hs.updated_at, 
							s.id, s.service_name, s.active, s.icon, s.created_at, s.updated_at, hs.last_message
						FROM
							host_services hs
							Left Join services s on (s.id = hs.service_id)
						WHERE
							hs.host_id = $1
						`
		var hostServices []models.HostServices
		serviceRows, err := m.DB.QueryContext(ctx, serviceQuery, host.ID)
		if err != nil {
			return hosts, err
		}
		for serviceRows.Next() {
			var hs models.HostServices
			err = serviceRows.Scan(
				&hs.ID,
				&hs.HostID,
				&hs.ServiceID,
				&hs.SecheduleNumber,
				&hs.ScheduleUnit,
				&hs.LastCheck,
				&hs.Active,
				&hs.Status,
				&hs.CreatedAt,
				&hs.UpdatedAt,
				&hs.Service.ID,
				&hs.Service.ServiceName,
				&hs.Service.Active,
				&hs.Service.Icon,
				&hs.Service.CreatedAt,
				&hs.Service.UpdatedAt,
				&hs.LastMessage,
			)
			if err != nil {
				return hosts, err
			}
			hostServices = append(hostServices, hs)
			//serviceRows.Close()
		}
		if err = serviceRows.Err(); err != nil {
			return hosts, err
		}
		err = serviceRows.Close()
		if err != nil {
			return nil, err
		}
		host.HostServices = hostServices
		hosts = append(hosts, host)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
		return hosts, err
	}
	return hosts, nil
}

func (m *postgresDBRepo) UpdateHostServiceStatus(hostId, serviceId, active int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stmt := `UPDATE host_services SET active=$1 where host_id=$2 and service_id=$3 `

	_, err := m.DB.ExecContext(ctx, stmt, active, hostId, serviceId)
	if err != nil {
		return err
	}
	return nil
}

func (m *postgresDBRepo) GetAllServiceStatusCounts() (int, int, int, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := ` SELECT
				(select count(id) from host_services where active=1 and status = 'pending') as pending,
				(select count(id) from host_services where active=1 and status = 'healthy') as healthy,
				(select count(id) from host_services where active=1 and status = 'warning') as warning,
				(select count(id) from host_services where active=1 and status = 'problem') as problem`

	var pending, healthy, warning, problem int
	row := m.DB.QueryRowContext(ctx, query)
	err := row.Scan(
		&pending,
		&healthy,
		&warning,
		&problem,
	)
	if err != nil {
		log.Println(err)
		return 0, 0, 0, 0, err
	}
	return pending, healthy, warning, problem, nil
}

func (m *postgresDBRepo) GetServiceByStatus(status string) ([]models.HostServices, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT
				hs.id, hs.host_id, hs.service_id, hs.schedule_number, hs.schedule_unit,
				hs.last_check, hs.status, hs.active, hs.created_at, hs.updated_at,
				s.service_name,
				h.host_name, hs.last_message
			FROM
				host_services hs
				LEFT JOIN services s on(s.id = hs.service_id)
				LEFT JOIN hosts h on(h.id = hs.host_id)
			WHERE
				hs.status = $1
				and hs.active = 1
			Order By
				host_name, service_name`

	var hostServices []models.HostServices
	rows, err := m.DB.QueryContext(ctx, query, status)
	if err != nil {
		log.Println(err)
		return hostServices, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	for rows.Next() {
		var hs models.HostServices
		err = rows.Scan(
			&hs.ID,
			&hs.HostID,
			&hs.ServiceID,
			&hs.SecheduleNumber,
			&hs.ScheduleUnit,
			&hs.LastCheck,
			&hs.Status,
			&hs.Active,
			&hs.CreatedAt,
			&hs.UpdatedAt,
			&hs.Service.ServiceName,
			&hs.HostName,
			&hs.LastMessage,
		)
		if err != nil {
			log.Println(err)
			return hostServices, err
		}
		hostServices = append(hostServices, hs)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
		return hostServices, err
	}
	return hostServices, nil
}

func (m *postgresDBRepo) GetServiceByID(id int) (models.HostServices, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var hs models.HostServices

	query := `SELECT 
					hs.id, hs.host_id, hs.service_id, hs.schedule_number, hs.schedule_unit, hs.last_check,
					hs.status, hs.active, hs.created_at, hs.updated_at,
					s.id, s.service_name, s.active, s.icon, s.created_at, s.updated_at,
       				h.host_name, hs.last_message
				FROM
					host_services hs
					LEFT JOIN services s on (s.id = hs.service_id)
					LEFT JOIN hosts h on (h.id = hs.host_id) 
				WHERE
					hs.id = $1`

	row := m.DB.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&hs.ID,
		&hs.HostID,
		&hs.ServiceID,
		&hs.SecheduleNumber,
		&hs.ScheduleUnit,
		&hs.LastCheck,
		&hs.Status,
		&hs.Active,
		&hs.CreatedAt,
		&hs.UpdatedAt,
		&hs.Service.ID,
		&hs.Service.ServiceName,
		&hs.Service.Active,
		&hs.Service.Icon,
		&hs.Service.CreatedAt,
		&hs.Service.UpdatedAt,
		&hs.HostName,
		&hs.LastMessage,
	)
	if err != nil {
		log.Println(err)
		return hs, err
	}
	return hs, nil

}

func (m *postgresDBRepo) UpdateHostService(hs models.HostServices) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `update host_services
			set host_id=$1, service_id=$2, schedule_number=$3, schedule_unit=$4,
			last_check=$5, status=$6, active=$7, updated_at=$8, last_message = $9
		where
			id=$10`
	_, err := m.DB.ExecContext(ctx, query,
		hs.HostID,
		hs.ServiceID,
		hs.SecheduleNumber,
		hs.ScheduleUnit,
		hs.LastCheck,
		hs.Status,
		hs.Active,
		hs.UpdatedAt,
		hs.LastMessage,
		hs.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (m *postgresDBRepo) GetServicesToMonitor() ([]models.HostServices, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var hostServices []models.HostServices

	query := `SELECT
				hs.id, hs.host_id, hs.service_id, hs.schedule_number, hs.schedule_unit,
				hs.last_check, hs.active, hs.status, hs.created_at, hs.updated_at,
				s.id, s.service_name, s.active, s.icon, s.updated_at, s.created_at,
				h.host_name, hs.last_message
			FROM
				host_services hs
				Left Join services s on (s.id = hs.service_id)
				Left Join hosts h on (h.id = hs.host_id)
			WHERE
				h.active = 1
				and hs.active = 1
`
	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return hostServices, err
	}
	for rows.Next() {
		var hs models.HostServices
		err = rows.Scan(
			&hs.ID,
			&hs.HostID,
			&hs.ServiceID,
			&hs.SecheduleNumber,
			&hs.ScheduleUnit,
			&hs.LastCheck,
			&hs.Active,
			&hs.Status,
			&hs.CreatedAt,
			&hs.UpdatedAt,
			&hs.Service.ID,
			&hs.Service.ServiceName,
			&hs.Service.Active,
			&hs.Service.Icon,
			&hs.Service.UpdatedAt,
			&hs.Service.CreatedAt,
			&hs.HostName,
			&hs.LastMessage)
		if err != nil {
			return hostServices, err
		}
		hostServices = append(hostServices, hs)
	}
	if err = rows.Err(); err != nil {
		return hostServices, err
	}
	return hostServices, nil
}

func (m *postgresDBRepo) GetHostServiceByHostIDServiceID(hostID, serviceID int) (models.HostServices, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var hs models.HostServices

	query := ` SELECT
				hs.id, hs.host_id, hs.service_id, hs.schedule_number, hs.schedule_unit, hs.last_check,
				hs.status, hs.active, hs.created_at, hs.updated_at,
				s.id, s.service_name, s.active, s.icon, s.created_at, s.updated_at,
				h.host_name, hs.last_message
			FROM
				host_services hs
				LEFT JOIN services s on (s.id = hs.service_id)
				LEFT JOIN hosts h on (h.id = hs.host_id)
			WHERE
				hs.host_id = $1 and hs.service_id = $2`

	row := m.DB.QueryRowContext(ctx, query, hostID, serviceID)
	err := row.Scan(
		&hs.ID,
		&hs.HostID,
		&hs.ServiceID,
		&hs.SecheduleNumber,
		&hs.ScheduleUnit,
		&hs.LastCheck,
		&hs.Status,
		&hs.Active,
		&hs.CreatedAt,
		&hs.UpdatedAt,
		&hs.Service.ID,
		&hs.Service.ServiceName,
		&hs.Service.Active,
		&hs.Service.Icon,
		&hs.Service.CreatedAt,
		&hs.Service.UpdatedAt,
		&hs.HostName,
		&hs.LastMessage)
	if err != nil {
		return hs, err
	}
	return hs, nil
}

func (m *postgresDBRepo) InsertEvents(e models.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stmt := `INSERT INTO 
				events(event_type, host_service_id, host_id, service_name, host_name, message,
				created_at, updated_at)
			VALUES
				($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := m.DB.ExecContext(ctx, stmt,
		e.EventType, e.HostServiceID, e.HostID, e.ServiceName, e.HostName, e.Message,
		time.Now(), time.Now())
	if err != nil {
		return err
	}
	return nil
}

func (m *postgresDBRepo) GetAllEvents() ([]models.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := ` SELECT
				id, event_type, host_service_id, host_id, service_name, host_name, message,
				created_at, updated_at
			FROM
				events`

	var events []models.Event

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return events, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	for rows.Next() {
		var ev models.Event
		err = rows.Scan(
			&ev.ID, &ev.EventType, &ev.HostServiceID, &ev.HostID, &ev.ServiceName, &ev.HostName,
			&ev.Message, &ev.CreatedAt, &ev.UpdatedAt)
		if err != nil {
			return events, err
		}
		events = append(events, ev)
	}

	if err = rows.Err(); err != nil {
		return events, err
	}
	return events, nil
}

package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/tsawler/vigilate/internal/channeldata"
	"github.com/tsawler/vigilate/internal/helpers"
	"github.com/tsawler/vigilate/internal/models"
	"github.com/tsawler/vigilate/internal/sms"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	HTTP  = 1
	HTTPS = 2
)

type jsonResp struct {
	OK            bool      `json:"ok"`
	Message       string    `json:"message"`
	ServiceID     int       `json:"service_id"`
	HostServiceID int       `json:"host_service_id"`
	HostID        int       `json:"host_id"`
	OldStatus     string    `json:"old_status"`
	NewStatus     string    `json:"new_status"`
	LastCheck     time.Time `json:"last_check"`
}

//ScheduleCheck checks the schedule details for a specific host service
func (repo *DBRepo) ScheduleCheck(hostServiceID int) {
	log.Println("schedule check for host service", hostServiceID)

	hs, err := repo.DB.GetServiceByID(hostServiceID)
	if err != nil {
		log.Println(err)
		return
	}

	h, err := repo.DB.GetHostById(hs.HostID)
	if err != nil {
		log.Println(err)
		return
	}

	//tests the service
	msg, newStatus := repo.testServiceForHost(h, hs)

	if hs.Status != newStatus {
		repo.updateHostServiceStatusCount(h, hs, newStatus, msg)
	}
}

func (repo *DBRepo) updateHostServiceStatusCount(h models.Host, hs models.HostServices, newStatus, msg string) {
	hs.Status = newStatus
	hs.LastMessage = msg
	hs.LastCheck = time.Now()

	//Update host service status in DB
	err := repo.DB.UpdateHostService(hs)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("new status is", newStatus, "and message is: ", msg)
	pending, healthy, warning, problem, err := repo.DB.GetAllServiceStatusCounts()
	if err != nil {
		log.Println(err)
		return
	}

	var payload = make(map[string]string)
	payload["pending"] = strconv.Itoa(pending)
	payload["healthy"] = strconv.Itoa(healthy)
	payload["warning"] = strconv.Itoa(warning)
	payload["problem"] = strconv.Itoa(problem)

	repo.App.WsClient.Trigger("public-channel", "host-service-count-changed", payload)

}

func (repo *DBRepo) TestCheck(w http.ResponseWriter, r *http.Request) {
	hostServiceID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	oldStatus := chi.URLParam(r, "oldStatus")
	okay := true

	log.Println(hostServiceID, oldStatus)

	hs, err := repo.DB.GetServiceByID(hostServiceID)
	if err != nil {
		log.Println(err)
		okay = false
	}

	h, err := repo.DB.GetHostById(hs.HostID)
	if err != nil {
		log.Println(err)
		okay = false
	}

	msg, newStatus := repo.testServiceForHost(h, hs)

	if newStatus != hs.Status {
		repo.pushHostStatusChangeEvent(h, hs, newStatus, msg)

		event := models.Event{
			EventType:     newStatus,
			HostServiceID: hs.ID,
			HostID:        hs.HostID,
			ServiceName:   hs.Service.ServiceName,
			HostName:      hs.HostName,
			Message:       msg,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		err := repo.DB.InsertEvents(event)
		if err != nil {
			log.Println(err)
		}
	}
	hs.Status = newStatus
	hs.LastMessage = msg
	hs.LastCheck = time.Now()
	hs.UpdatedAt = time.Now()
	err = repo.DB.UpdateHostService(hs)
	if err != nil {
		log.Println(err)
		okay = false
	}

	var resp jsonResp
	if !okay {
		resp.OK = okay
		resp.Message = "something went wrong"
	} else {
		resp = jsonResp{
			OK:            okay,
			Message:       msg,
			ServiceID:     hs.ServiceID,
			HostServiceID: hs.ID,
			HostID:        hs.HostID,
			OldStatus:     oldStatus,
			NewStatus:     newStatus,
			LastCheck:     time.Now(),
		}
	}
	out, _ := json.MarshalIndent(resp, "", "   ")

	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (repo *DBRepo) testServiceForHost(h models.Host, hs models.HostServices) (string, string) {
	var msg, newStatus string

	switch hs.ServiceID {
	case HTTP:
		msg, newStatus = repo.testHTTPForHost(h.URL)
		break
	}

	if hs.Status != newStatus {
		repo.pushHostStatusChangeEvent(h, hs, newStatus, msg)

		event := models.Event{
			EventType:     newStatus,
			HostServiceID: hs.ID,
			HostID:        hs.HostID,
			ServiceName:   hs.Service.ServiceName,
			HostName:      hs.HostName,
			Message:       msg,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		err := repo.DB.InsertEvents(event)
		if err != nil {
			log.Println(err)
		}

		//Send Email
		if repo.App.PreferenceMap["notify_via_email"] == "1" {
			if hs.Status != "pending" {
				mm := channeldata.MailData{
					ToName:    repo.App.PreferenceMap["notify_name"],
					ToAddress: repo.App.PreferenceMap["notify_email"],
				}

				if newStatus == "healthy" {
					mm.Subject = fmt.Sprintf("Healthy: service %s on %s", hs.Service.ServiceName, hs.HostName)
					mm.Content = template.HTML(fmt.Sprintf(`<p>Service %s on %s reported healthy</p>
								<p><strong>Message Received: %s</strong></p>`, hs.Service.ServiceName, hs.HostName, msg))
				} else if newStatus == "problem" {
					mm.Subject = fmt.Sprintf("Problem: service %s on %s", hs.Service.ServiceName, hs.HostName)
					mm.Content = template.HTML(fmt.Sprintf(`<p>Service %s on %s reported problem</p>
								<p><strong>Message Received: %s</strong></p>`, hs.Service.ServiceName, hs.HostName, msg))
				} else if newStatus == "warning" {
					mm.Subject = fmt.Sprintf("Warning: service %s on %s", hs.Service.ServiceName, hs.HostName)
					mm.Content = template.HTML(fmt.Sprintf(`<p>Service %s on %s reported warning</p>
								<p><strong>Message Received: %s</strong></p>`, hs.Service.ServiceName, hs.HostName, msg))
				}

				helpers.SendEmail(mm)

			}
		}
		//
		if repo.App.PreferenceMap["notify_via_sms"] == "1" {
			to := repo.App.PreferenceMap["sms_notify_number"]
			smsMsg := ""
			if newStatus == "healthy" {
				smsMsg = fmt.Sprintf("%s on %s is healthy", hs.Service.ServiceName, hs.HostName)
			} else if newStatus == "warning" {
				smsMsg = fmt.Sprintf("%s on %s reports warning: %s", hs.Service.ServiceName, hs.HostName, msg)
			} else if newStatus == "problem" {
				smsMsg = fmt.Sprintf("%s on %s reports problem: %s", hs.Service.ServiceName, hs.HostName, msg)
			}
			err := sms.SendTextTwilio(to, smsMsg, repo.App)
			if err != nil {
				log.Println("error sending sms in perform-checks", err)
			}
		}
	}

	repo.pushScheduleChangeEvent(hs, newStatus)

	return msg, newStatus
}

func (repo *DBRepo) pushHostStatusChangeEvent(h models.Host, hs models.HostServices, newStatus, msg string) {
	var data = make(map[string]string)
	data["host_service_id"] = strconv.Itoa(hs.ID)
	data["host_id"] = strconv.Itoa(hs.HostID)
	data["host_name"] = h.HostName
	data["service_name"] = hs.Service.ServiceName
	data["icon"] = hs.Service.Icon
	data["status"] = newStatus
	data["last_message"] = msg
	data["last_check"] = time.Now().Format("2006-01-02 03:04:05 PM")
	data["message"] = fmt.Sprintf("The service %s on %s has changed to %s", hs.Service.ServiceName, h.HostName, newStatus)
	repo.broadcastMessage("public-channel", "host-service-status-changed", data)
}

func (repo *DBRepo) pushScheduleChangeEvent(hs models.HostServices, newStatus string) {
	//Schedule change event
	yearOne := time.Date(0001, 01, 01, 01, 00, 00, 00, time.UTC)
	var data = make(map[string]string)
	data["host_id"] = strconv.Itoa(hs.HostID)
	data["host_service_id"] = strconv.Itoa(hs.ID)
	data["service_id"] = strconv.Itoa(hs.ServiceID)
	data["service"] = hs.Service.ServiceName
	data["icon"] = hs.Service.Icon
	data["status"] = newStatus

	if repo.App.Scheduler.Entry(repo.App.MonitorMap[hs.ID]).Next.After(yearOne) {
		data["next_run"] = repo.App.Scheduler.Entry(repo.App.MonitorMap[hs.ID]).Next.Format("2006-01-02 03:04:05 PM")
	} else {
		data["next_run"] = "Pending..."
	}

	data["last_run"] = time.Now().Format("2006-01-02 03:04:05 PM")
	data["host"] = hs.HostName
	data["schedule"] = fmt.Sprintf("@every %d%s", hs.SecheduleNumber, hs.ScheduleUnit)
	repo.broadcastMessage("public-channel", "schedule-change-event", data)
}

func (repo *DBRepo) testHTTPForHost(url string) (string, string) {
	if strings.HasSuffix(url, "/") {
		url = strings.TrimSuffix(url, "/")
	}

	url = strings.Replace(url, "https://", "http://", -1)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("%s - %s", url, "error connecting"), "problem"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("%s - %s", url, resp.Status), "problem"
	}
	return fmt.Sprintf("%s - %s", url, resp.Status), "healthy"
}

func (repo *DBRepo) broadcastMessage(channel, event string, data map[string]string) {
	err := repo.App.WsClient.Trigger(channel, event, data)
	if err != nil {
		log.Println(err)
	}
}

func (repo *DBRepo) addToMonitorMap(hs models.HostServices) {
	if repo.App.PreferenceMap["monitoring_live"] == "1" {
		var j job
		j.HostServiceID = hs.ID

		log.Println("host service id", hs.ID)
		var sch string
		if hs.ScheduleUnit == "d" {
			sch = fmt.Sprintf("@every %d%s", hs.SecheduleNumber*24, "h")
		} else {
			sch = fmt.Sprintf("@every %d%s", hs.SecheduleNumber, hs.ScheduleUnit)
		}

		scheduleID, err := repo.App.Scheduler.AddJob(sch, j)
		if err != nil {
			log.Println(err)
		}

		repo.App.MonitorMap[hs.ID] = scheduleID
		data := make(map[string]string)
		data["message"] = "scheduling"
		data["host_service_id"] = strconv.Itoa(hs.ID)
		data["next_run"] = "Pending..."
		data["service"] = hs.Service.ServiceName
		data["host"] = hs.HostName
		data["last_run"] = hs.LastCheck.Format("2006-01-02 03:04:05 PM")
		data["schedule"] = sch
		repo.broadcastMessage("public-channel", "schedule-change-event", data)
	}
}

func (repo *DBRepo) removeFromMonitorMap(hs models.HostServices) {
	if repo.App.PreferenceMap["monitoring_live"] == "1" {
		repo.App.Scheduler.Remove(repo.App.MonitorMap[hs.ID])

		data := make(map[string]string)
		data["host_service_id"] = strconv.Itoa(hs.ID)
		repo.broadcastMessage("public-channel", "schedule-item-removed-event", data)
	}
}

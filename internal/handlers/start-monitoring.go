package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

type job struct {
	HostServiceID int
}

func (j job) Run() {
	Repo.ScheduleCheck(j.HostServiceID)
}

func (repo *DBRepo) StartMonitoring() {
	if app.PreferenceMap["monitoring_live"] == "1" {
		//trigger a message to broadcast to all clients that the app is starting to monitor
		data := make(map[string]string)
		data["message"] = "Monitoring is starting..."
		err := app.WsClient.Trigger("public-channel", "app-starting", data)
		if err != nil {
			log.Println(err)
		}

		//get all services to monitor from db
		servicesToMonitor, err := repo.DB.GetServicesToMonitor()
		if err != nil {
			log.Println(err)
			return
		}

		//range through the services
		for _, x := range servicesToMonitor {
			var sch string
			//get the schedule unit and number
			if x.ScheduleUnit == "d" {
				sch = fmt.Sprintf("@every %d%s", x.SecheduleNumber*24, "h")
			} else {
				sch = fmt.Sprintf("@every %d%s", x.SecheduleNumber, x.ScheduleUnit)
			}

			//create a job
			var j job
			j.HostServiceID = x.ID

			//save the id of the job so we can start/stop it
			scheduleID, err := app.Scheduler.AddJob(sch, j)
			if err != nil {
				log.Println(err)
			}
			app.MonitorMap[x.ID] = scheduleID

			//broadcast over websockets the fact that the service is scheduled
			var payload = make(map[string]string)
			payload["message"] = "scheduling"
			payload["host_service_id"] = strconv.Itoa(x.ID)
			//any time after start date
			yearOne := time.Date(0001, 11, 17, 20, 34, 58, 65138737, time.UTC)
			//If we are running the job for the first time, the status will be pending but if the job
			//has been stopped and restarted while the application is running, there might be next schedule
			if app.Scheduler.Entry(app.MonitorMap[x.ID]).Next.After(yearOne) {
				payload["next_run"] = app.Scheduler.Entry(app.MonitorMap[x.ID]).Next.Format("2006-01-02 3:04:05 PM")
			} else {
				payload["next_run"] = "pending..."
			}
			payload["host"] = x.HostName
			payload["service"] = x.Service.ServiceName
			//If the last run is after the yearone then the job has run in the past or else it has not
			if x.LastCheck.After(yearOne) {
				payload["last_run"] = x.LastCheck.Format("2006-01-02 3:04:05 PM")
			} else {
				payload["last_run"] = "pending"
			}
			payload["schedule"] = fmt.Sprintf("@every %d%s", x.SecheduleNumber, x.ScheduleUnit)

			err = app.WsClient.Trigger("public-channel", "next-run-event", payload)
			if err != nil {
				log.Println(err)
			}

			err = app.WsClient.Trigger("public-channel", "schedule-change-event", payload)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

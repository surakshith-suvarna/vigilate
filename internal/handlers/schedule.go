package handlers

import (
	"fmt"
	"github.com/CloudyKit/jet/v6"
	"github.com/tsawler/vigilate/internal/helpers"
	"github.com/tsawler/vigilate/internal/models"
	"log"
	"net/http"
	"sort"
)

type ByHost []models.Schedule

//Len returns the length of map
func (b ByHost) Len() int { return len(b) }

//Less is used to sort by host
func (b ByHost) Less(i, j int) bool {
	return b[i].Host < b[j].Host
}

//Swap is used to sort by host
func (b ByHost) Swap(i, j int) {
	b[i], b[j] = b[i], b[j]
}

// ListEntries lists schedule entries
func (repo *DBRepo) ListEntries(w http.ResponseWriter, r *http.Request) {

	var schedules []models.Schedule
	for k, v := range repo.App.MonitorMap {
		var schedule models.Schedule
		schedule.ID = k
		schedule.EntryID = v
		schedule.Entry = repo.App.Scheduler.Entry(v)
		//schedule.ScheduleNext = repo.App.Scheduler.Entry(v).Next.Format("2006-01-02 03:04:05 PM")

		hostService, err := repo.DB.GetServiceByID(k)
		if err != nil {
			log.Println(err)
		}
		schedule.Schedule = fmt.Sprintf("@every %d%s", hostService.SecheduleNumber, hostService.ScheduleUnit)
		schedule.Host = hostService.HostName
		schedule.Service = hostService.Service.ServiceName
		schedule.HostServiceID = hostService.ID
		schedule.LastRunFromHS = hostService.LastCheck

		schedules = append(schedules, schedule)

	}

	sort.Sort(ByHost(schedules))

	varMap := make(jet.VarMap)
	varMap.Set("schedules", schedules)

	err := helpers.RenderPage(w, r, "schedule", varMap, nil)
	if err != nil {
		printTemplateError(w, err)
	}
}

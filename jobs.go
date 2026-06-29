package writefreely

import (
	"github.com/writeas/web-core/log"
	"time"
)

type PostJob struct {
	ID     int64
	PostID string
	Action string
	Delay  int64
}

func addJob(app *App, p *PublicPost, action string, delay int64) error {
	j := &PostJob{
		PostID: p.ID,
		Action: action,
		Delay:  delay,
	}
	return app.db.InsertJob(j)
}

func startPublishJobsQueue(app *App) {
	t := time.NewTicker(62 * time.Second)
	for {
		log.Info("[jobs] Done.")
		<-t.C
		log.Info("[jobs] Fetching email publish jobs...")
		jobs, err := app.db.GetJobsToRun("email")
		if err != nil {
			log.Error("[jobs] %s - Skipping.", err)
			continue
		}
		log.Info("[jobs] Running %d email publish jobs...", len(jobs))
		err = runJobs(app, jobs, true)
		if err != nil {
			log.Error("[jobs] Failed: %s", err)
		}
	}
}

func runJobs(app *App, jobs []*PostJob, reqColl bool) error {
	for _, j := range jobs {
		p, err := app.db.GetPost(j.PostID, 0)
		if err != nil {
			log.Info("[job #%d] Unable to get post: %s", j.ID, err)
			continue
		}
		if !p.CollectionID.Valid && reqColl {
			log.Info("[job #%d] Post %s not part of a collection", j.ID, p.ID)
			app.db.DeleteJob(j.ID)
			continue
		}
		coll, err := app.db.GetCollectionByID(p.CollectionID.Int64)
		if err != nil {
			log.Info("[job #%d] Unable to get collection: %s", j.ID, err)
			continue
		}
		coll.hostName = app.cfg.App.Host
		coll.ForPublic()
		p.Collection = &CollectionObj{Collection: *coll}
		err = emailPost(app, p, p.Collection.ID)
		if err != nil {
			log.Error("[job #%d] Failed to email post %s", j.ID, p.ID)
			continue
		}
		log.Info("[job #%d] Success for post %s.", j.ID, p.ID)
		app.db.DeleteJob(j.ID)
	}
	return nil
}

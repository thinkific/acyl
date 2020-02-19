package eventlogger

import (
	"time"

	"github.com/dollarshaveclub/acyl/pkg/models"
)

func (l *Logger) SetInitialStatus(rc *models.RepoConfig, status models.EventStatus, processingTime time.Duration) {
	if rc == nil {
		l.Printf("error setting initial status: rc is nil")
		return
	}
	rm, err := rc.RefMap()
	if err != nil {
		l.Printf("error setting initial status: error getting refmap: %v", err)
		return
	}
	out := models.EventStatusSummary{
		Config: models.EventStatusSummaryConfig{
			Status:         status,
			Started:        time.Now().UTC(),
			ProcessingTime: processingTime,
			RefMap:         rm,
		},
		Tree: make(map[string]models.EventStatusTreeNode, rc.Dependencies.Count()+1),
	}
	out.Tree[models.GetName(rc.Application.Repo)] = models.EventStatusTreeNode{
		Chart: models.EventStatusTreeNodeChart{
			Status: models.WaitingChartStatus,
		},
		Image: models.EventStatusTreeNodeImage{
			Name: rc.Application.Image,
		},
	}
	for _, dep := range rc.Dependencies.All() {
		node := models.EventStatusTreeNode{
			Parent: dep.Parent,
			Chart: models.EventStatusTreeNodeChart{
				Status: models.WaitingChartStatus,
			},
		}
		if dep.Repo != "" {
			node.Image.Name = dep.AppMetadata.Image
		}
		out.Tree[dep.Name] = node
	}
	if err := l.DL.SetEventStatus(l.ID, out); err != nil {
		l.Printf("error setting event status in db: %v", err)
	}
}

func (l *Logger) SetImageStarted(name string) {
	if err := l.DL.SetEventStatusImageStarted(l.ID, name); err != nil {
		l.Printf("error setting image status to started: %v: %v", name, err)
	}
}

func (l *Logger) SetImageCompleted(name string, err bool) {
	if err2 := l.DL.SetEventStatusImageCompleted(l.ID, name, err); err2 != nil {
		l.Printf("error setting image status to completed: %v: %v", name, err2)
	}
}

func (l *Logger) SetChartStarted(name string, status models.NodeChartStatus) {
	if err := l.DL.SetEventStatusChartStarted(l.ID, name, status); err != nil {
		l.Printf("error setting chart status to started: %v: %v", name, err)
	}
}

func (l *Logger) SetChartCompleted(name string, status models.NodeChartStatus) {
	if err := l.DL.SetEventStatusChartCompleted(l.ID, name, status); err != nil {
		l.Printf("error setting chart status to completed: %v: %v", name, err)
	}
}

func (l *Logger) SetCompletedStatus(status models.EventStatus) {
	if err := l.DL.SetEventStatusCompleted(l.ID, status); err != nil {
		l.Printf("error setting event status to completed: %v", err)
	}
}

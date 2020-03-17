package persistence

import (
	"log"
	"math/rand"
	"os"
	"time"

	lorem "github.com/dollarshaveclub/acyl/pkg/persistence/golorem"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/namegen"
	"github.com/google/uuid"
)

func randomDuration(minMillis, maxMillis uint) time.Duration {
	if maxMillis < minMillis {
		return time.Duration(0)
	}

	rand.Seed(time.Now().UnixNano())
	return time.Duration(rand.Intn(int(maxMillis-minMillis+1))+int(minMillis)) * time.Millisecond
}

func randomLogLines(n uint) []string {
	out := make([]string, n)
	for i := 0; i < int(n); i++ {
		out[i] = lorem.Sentence(5, 20)
	}
	return out
}

func (fdl *FakeDataLayer) updateEvent(id uuid.UUID) {
	// update config after a bit
	cfgd := randomDuration(50, 200)
	time.Sleep(cfgd)

	ng, _ := namegen.NewWordnetNameGenerator("../../data/words.json.gz", log.New(os.Stderr, "", log.LstdFlags))
	name := "foonly-receptacle"
	if ng != nil {
		rn, err := ng.New()
		if err == nil {
			name = rn
		}
	}

	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	fdl.data.elogs[id].Status.Config.EnvName = name
	fdl.data.elogs[id].Status.Config.ProcessingTime = models.ConfigProcessingDuration{Duration: cfgd}
	fdl.data.elogs[id].Status.Config.RefMap = map[string]string{"foo/somethingelse": "master", "foo/dependency": "feature-foo"}
	fdl.data.elogs[id].Status.Tree = map[string]models.EventStatusTreeNode{
		"foo-bar": models.EventStatusTreeNode{
			Parent: "",
			Image: models.EventStatusTreeNodeImage{
				Name:    "quay.io/foo/bar",
				Started: time.Now().UTC(),
			},
			Chart: models.EventStatusTreeNodeChart{
				Status: models.WaitingChartStatus,
			},
		},
		"foo-somethingelse": models.EventStatusTreeNode{
			Parent: "foo-bar",
			Image: models.EventStatusTreeNodeImage{
				Name:    "quay.io/foo/somethingelse",
				Started: time.Now().UTC(),
			},
			Chart: models.EventStatusTreeNodeChart{
				Status: models.WaitingChartStatus,
			},
		},
		"foo-dependency": models.EventStatusTreeNode{
			Parent: "foo-bar",
			Image: models.EventStatusTreeNodeImage{
				Name:    "quay.io/foo/dependency",
				Started: time.Now().UTC(),
			},
			Chart: models.EventStatusTreeNodeChart{
				Status: models.WaitingChartStatus,
			},
		},
		"foo-dependency-redis": models.EventStatusTreeNode{
			Parent: "foo-dependency",
			Chart: models.EventStatusTreeNodeChart{
				Status: models.WaitingChartStatus,
			},
		},
		"foo-dependency-postgres": models.EventStatusTreeNode{
			Parent: "foo-dependency",
			Chart: models.EventStatusTreeNodeChart{
				Status: models.WaitingChartStatus,
			},
		},
	}
	fdl.data.Unlock()

	// update leaf dependencies to installing
	time.Sleep(randomDuration(500, 5000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	c := fdl.data.elogs[id].Status.Tree["foo-dependency-redis"]
	c.Chart.Status = models.InstallingChartStatus
	c.Chart.Started = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-dependency-redis"] = c
	c = fdl.data.elogs[id].Status.Tree["foo-dependency-postgres"]
	c.Chart.Status = models.InstallingChartStatus
	c.Chart.Started = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-dependency-postgres"] = c
	fdl.data.Unlock()

	// complete leaf dependencies
	time.Sleep(randomDuration(0, 3000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	c = fdl.data.elogs[id].Status.Tree["foo-dependency-redis"]
	c.Chart.Status = models.DoneChartStatus
	c.Chart.Completed = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-dependency-redis"] = c
	c = fdl.data.elogs[id].Status.Tree["foo-dependency-postgres"]
	c.Chart.Status = models.DoneChartStatus
	c.Chart.Completed = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-dependency-postgres"] = c
	fdl.data.Unlock()

	// finish foo-dependency image build & start installing
	time.Sleep(randomDuration(0, 3000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	i := fdl.data.elogs[id].Status.Tree["foo-dependency"]
	i.Image.Completed = time.Now().UTC()
	i.Image.Error = false
	i.Chart.Status = models.InstallingChartStatus
	i.Chart.Started = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-dependency"] = i
	fdl.data.Unlock()

	// finish foo-somethingelse image build & start installing
	time.Sleep(randomDuration(0, 4000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	i = fdl.data.elogs[id].Status.Tree["foo-somethingelse"]
	i.Image.Completed = time.Now().UTC()
	i.Image.Error = false
	i.Chart.Status = models.InstallingChartStatus
	i.Chart.Started = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-somethingelse"] = i
	fdl.data.Unlock()

	// complete foo-bar image build, set chart to waiting
	time.Sleep(randomDuration(0, 5000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	i = fdl.data.elogs[id].Status.Tree["foo-bar"]
	i.Image.Completed = time.Now().UTC()
	i.Image.Error = false
	i.Chart.Status = models.WaitingChartStatus
	fdl.data.elogs[id].Status.Tree["foo-bar"] = i
	fdl.data.Unlock()

	// complete foo-dependency install
	time.Sleep(randomDuration(0, 4000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	i = fdl.data.elogs[id].Status.Tree["foo-dependency"]
	i.Chart.Status = models.DoneChartStatus
	i.Chart.Completed = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-dependency"] = i
	fdl.data.Unlock()

	// complete foo-somethingelse install
	time.Sleep(randomDuration(0, 3000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	i = fdl.data.elogs[id].Status.Tree["foo-somethingelse"]
	i.Chart.Status = models.DoneChartStatus
	i.Chart.Completed = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-somethingelse"] = i
	fdl.data.Unlock()

	// start foo-bar install
	time.Sleep(randomDuration(0, 3000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	i = fdl.data.elogs[id].Status.Tree["foo-bar"]
	i.Chart.Status = models.InstallingChartStatus
	i.Chart.Started = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-bar"] = i
	fdl.data.Unlock()

	// complete foo-bar install, mark event as completed
	time.Sleep(randomDuration(1000, 2000))
	fdl.data.Lock()
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, randomLogLines(10)...)
	i = fdl.data.elogs[id].Status.Tree["foo-bar"]
	i.Chart.Status = models.DoneChartStatus
	//i.Chart.Status = models.FailedChartStatus
	i.Chart.Completed = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree["foo-bar"] = i
	fdl.data.elogs[id].Status.Config.Status = models.DoneStatus
	//fdl.data.elogs[id].Status.Config.Status = models.FailedStatus
	fdl.data.elogs[id].Status.Config.Completed = time.Now().UTC()
	fdl.data.Unlock()
}

func (fdl *FakeDataLayer) newStatus(id uuid.UUID) *models.EventStatusSummary {
	out := models.EventStatusSummary{
		Config: models.EventStatusSummaryConfig{
			Type:           models.CreateEvent,
			Status:         models.PendingStatus,
			TriggeringRepo: "foo/bar",
			PullRequest:    23,
			GitHubUser:     "john.smith",
			Branch:         "feature-foo",
			Revision:       "asdf1234",
			Started:        time.Now().UTC(),
		},
		Tree: map[string]models.EventStatusTreeNode{},
	}

	fdl.data.Lock()
	if fdl.data.elogs[id] == nil {
		fdl.data.elogs[id] = &models.EventLog{}
	}
	fdl.data.elogs[id].Status = out
	fdl.data.elogs[id].Log = []string{}
	fdl.data.Unlock()

	go fdl.updateEvent(id)
	return &out
}

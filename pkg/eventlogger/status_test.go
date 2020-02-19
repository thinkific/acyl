package eventlogger

import (
	"os"
	"testing"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/google/uuid"
)

var testRC = models.RepoConfig{
	Version:        2,
	TargetBranches: []string{"master"},
	Application: models.RepoConfigAppMetadata{
		Repo:   "foo/bar",
		Image:  "quay.io/foo/bar",
		Ref:    "asdf",
		Branch: "master",
	},
	Dependencies: models.DependencyDeclaration{
		Direct: []models.RepoConfigDependency{
			models.RepoConfigDependency{
				Repo: "foo/something",
				AppMetadata: models.RepoConfigAppMetadata{
					Repo:   "foo/something",
					Image:  "quay.io/foo/something",
					Ref:    "1234",
					Branch: "master",
				},
				Parent:   models.GetName("foo/bar"),
				Requires: []string{"otherthing"},
			},
			models.RepoConfigDependency{
				Name:   "otherthing",
				Parent: "foo/something",
			},
		},
	},
}

func TestSetInitialStatus(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetInitialStatus(&testRC, models.CreateNewStatus, 10*time.Millisecond)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if status := el2.Config.Status; status != models.CreateNewStatus {
		t.Fatalf("unexpected status: %v", status.String())
	}

	if n := len(el2.Tree); n != 3 {
		t.Fatalf("unexpected tree size: %v", n)
	}

	if _, ok := el2.Tree["otherthing"]; !ok {
		t.Fatalf("missing tree node: %+v", el2.Tree)
	}

	if node := el2.Tree["otherthing"]; node.Parent != "foo/something" {
		t.Fatalf("bad node: %+v", node)
	}
}

func TestSetImageStarted(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetInitialStatus(&testRC, models.CreateNewStatus, 10*time.Millisecond)

	elog.SetImageStarted("foo/something")

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Tree["foo/something"].Image.Started.IsZero() {
		t.Fatalf("image should have been started")
	}
}

func TestSetImageCompleted(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetInitialStatus(&testRC, models.CreateNewStatus, 10*time.Millisecond)

	elog.SetImageCompleted("foo/something", true)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Tree["foo/something"].Image.Completed.IsZero() {
		t.Fatalf("image should have been completed")
	}

	if !el2.Tree["foo/something"].Image.Error {
		t.Fatalf("image should have been marked as error")
	}
}

func TestSetChartStarted(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetInitialStatus(&testRC, models.CreateNewStatus, 10*time.Millisecond)

	elog.SetChartStarted("foo/something", models.InstallingChartStatus)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Tree["foo/something"].Chart.Started.IsZero() {
		t.Fatalf("chart should have been started")
	}

	if status := el2.Tree["foo/something"].Chart.Status; status != models.InstallingChartStatus {
		t.Fatalf("unexpected status: %v", status)
	}
}

func TestSetChartCompleted(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetInitialStatus(&testRC, models.CreateNewStatus, 10*time.Millisecond)

	elog.SetChartCompleted("foo/something", models.DoneChartStatus)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Tree["foo/something"].Chart.Completed.IsZero() {
		t.Fatalf("chart should have been completed")
	}

	if status := el2.Tree["foo/something"].Chart.Status; status != models.DoneChartStatus {
		t.Fatalf("unexpected status: %v", status)
	}
}

func TestSetCompletedStatus(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetInitialStatus(&testRC, models.CreateNewStatus, 10*time.Millisecond)

	elog.SetCompletedStatus(models.DoneStatus)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Config.Completed.IsZero() {
		t.Fatalf("config should have been completed")
	}

	if status := el2.Config.Status; status != models.DoneStatus {
		t.Fatalf("unexpected status: %v", status)
	}
}

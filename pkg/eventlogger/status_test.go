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
				Name: "something",
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
				Parent: "something",
			},
		},
	},
}

func TestSetNewStatus(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetNewStatus(models.CreateEvent)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if etype := el2.Config.Type; etype != models.CreateEvent {
		t.Fatalf("unexpected event type: %v", etype.String())
	}

	if el2.Config.Started.IsZero() {
		t.Fatalf("config should have been started")
	}
}

func TestSetInitialStatus(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetNewStatus(models.CreateEvent)

	elog.SetInitialStatus(&testRC, 10*time.Millisecond)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if status := el2.Config.Status; status != models.PendingStatus {
		t.Fatalf("unexpected status: %v", status.String())
	}

	if etype := el2.Config.Type; etype != models.CreateEvent {
		t.Fatalf("unexpected event type: %v", etype.String())
	}

	if n := len(el2.Tree); n != 3 {
		t.Fatalf("unexpected tree size: %v", n)
	}

	if _, ok := el2.Tree["otherthing"]; !ok {
		t.Fatalf("missing tree node: %+v", el2.Tree)
	}

	if node := el2.Tree["otherthing"]; node.Parent != "something" {
		t.Fatalf("bad node: %+v", node)
	}
}

func TestSetImageStarted(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetNewStatus(models.CreateEvent)

	elog.SetInitialStatus(&testRC, 10*time.Millisecond)

	elog.SetImageStarted("something")

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Tree["something"].Image.Started.IsZero() {
		t.Fatalf("image should have been started")
	}
}

func TestSetImageCompleted(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetNewStatus(models.CreateEvent)

	elog.SetInitialStatus(&testRC, 10*time.Millisecond)

	elog.SetImageCompleted("something", true)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Tree["something"].Image.Completed.IsZero() {
		t.Fatalf("image should have been completed")
	}

	if !el2.Tree["something"].Image.Error {
		t.Fatalf("image should have been marked as error")
	}
}

func TestSetChartStarted(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetNewStatus(models.CreateEvent)

	elog.SetInitialStatus(&testRC, 10*time.Millisecond)

	elog.SetChartStarted("something", models.InstallingChartStatus)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Tree["something"].Chart.Started.IsZero() {
		t.Fatalf("chart should have been started")
	}

	if status := el2.Tree["something"].Chart.Status; status != models.InstallingChartStatus {
		t.Fatalf("unexpected status: %v", status)
	}
}

func TestSetChartCompleted(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetNewStatus(models.CreateEvent)

	elog.SetInitialStatus(&testRC, 10*time.Millisecond)

	elog.SetChartCompleted("something", models.DoneChartStatus)

	el2, err := dl.GetEventStatus(id)
	if err != nil {
		t.Fatalf("error getting event status: %v", err)
	}

	if el2.Tree["something"].Chart.Completed.IsZero() {
		t.Fatalf("chart should have been completed")
	}

	if status := el2.Tree["something"].Chart.Status; status != models.DoneChartStatus {
		t.Fatalf("unexpected status: %v", status)
	}
}

func TestSetCompletedStatus(t *testing.T) {
	dl := persistence.NewFakeDataLayer()
	id, _ := uuid.NewRandom()
	elog := Logger{DL: dl, ID: id, Sink: os.Stderr}
	elog.Init([]byte{}, "foo/bar", 99)

	elog.SetNewStatus(models.CreateEvent)

	elog.SetInitialStatus(&testRC, 10*time.Millisecond)

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

package persistence

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/dollarshaveclub/acyl/pkg/models"
)

type lockingDataMap struct {
	sync.RWMutex
	// each field below represents a table
	d     map[string]*models.QAEnvironment
	helm  map[string][]models.HelmRelease
	k8s   map[string]*models.KubernetesEnvironment
	elogs map[uuid.UUID]*models.EventLog
}

// FakeDataLayer is a fake implementation of DataLayer that persists data in-memory, for testing purposes
type FakeDataLayer struct {
	data *lockingDataMap
}

var _ DataLayer = &FakeDataLayer{}

func NewFakeDataLayer() *FakeDataLayer {
	return &FakeDataLayer{
		data: &lockingDataMap{
			d:     make(map[string]*models.QAEnvironment),
			helm:  make(map[string][]models.HelmRelease),
			k8s:   make(map[string]*models.KubernetesEnvironment),
			elogs: make(map[uuid.UUID]*models.EventLog),
		},
	}
}

// NewPopulatedFakeDataLayer returns a FakeDataLayer populated with the supplied data. Input data is not checked for consistency.
func NewPopulatedFakeDataLayer(qaenvs []models.QAEnvironment, k8senvs []models.KubernetesEnvironment, helmreleases []models.HelmRelease) *FakeDataLayer {
	fd := NewFakeDataLayer()
	for i := range qaenvs {
		fd.data.d[qaenvs[i].Name] = &qaenvs[i]
	}
	for i := range k8senvs {
		fd.data.k8s[k8senvs[i].EnvName] = &k8senvs[i]
	}
	for _, hr := range helmreleases {
		fd.data.helm[hr.EnvName] = append(fd.data.helm[hr.EnvName], hr)
	}
	return fd
}

// Save writes all data to files in dir and returns the filenames, or error
func (fdl *FakeDataLayer) Save(dir string) ([]string, error) {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	names := make([]string, 4)
	// envs
	b, err := json.Marshal(fdl.data.d)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling envs")
	}
	f, err := os.Create(filepath.Join(dir, "envs.json"))
	if err != nil {
		return nil, errors.Wrap(err, "error creating envs.json")
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return nil, errors.Wrap(err, "error writing envs.json")
	}
	names[0] = f.Name()
	// helm
	b, err = json.Marshal(fdl.data.helm)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling helm")
	}
	f, err = os.Create(filepath.Join(dir, "helm.json"))
	if err != nil {
		return nil, errors.Wrap(err, "error creating helm.json")
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return nil, errors.Wrap(err, "error writing helm.json")
	}
	names[1] = f.Name()
	// k8s
	b, err = json.Marshal(fdl.data.k8s)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling k8s")
	}
	f, err = os.Create(filepath.Join(dir, "k8s.json"))
	if err != nil {
		return nil, errors.Wrap(err, "error creating k8s.json")
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return nil, errors.Wrap(err, "error writing k8s.json")
	}
	names[2] = f.Name()
	// elogs
	b, err = json.Marshal(fdl.data.elogs)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling elogs")
	}
	f, err = os.Create(filepath.Join(dir, "elogs.json"))
	if err != nil {
		return nil, errors.Wrap(err, "error creating elogs.json")
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return nil, errors.Wrap(err, "error writing elogs.json")
	}
	names[3] = f.Name()
	return names, nil
}

// Load reads JSON files in dir and loads them, overwriting any existing data
func (fdl *FakeDataLayer) Load(dir string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	b, err := ioutil.ReadFile(filepath.Join(dir, "envs.json"))
	if err != nil {
		return errors.Wrap(err, "error reading envs.json")
	}
	if err := json.Unmarshal(b, &fdl.data.d); err != nil {
		return errors.Wrap(err, "error unmarshaling envs.json")
	}
	b, err = ioutil.ReadFile(filepath.Join(dir, "helm.json"))
	if err != nil {
		return errors.Wrap(err, "error reading helm.json")
	}
	if err := json.Unmarshal(b, &fdl.data.helm); err != nil {
		return errors.Wrap(err, "error unmarshaling helm.json")
	}
	b, err = ioutil.ReadFile(filepath.Join(dir, "k8s.json"))
	if err != nil {
		return errors.Wrap(err, "error reading k8s.json")
	}
	if err := json.Unmarshal(b, &fdl.data.k8s); err != nil {
		return errors.Wrap(err, "error unmarshaling k8s.json")
	}
	b, err = ioutil.ReadFile(filepath.Join(dir, "elogs.json"))
	if err != nil {
		return errors.Wrap(err, "error reading elogs.json")
	}
	if err := json.Unmarshal(b, &fdl.data.elogs); err != nil {
		return errors.Wrap(err, "error unmarshaling elogs.json")
	}
	return nil
}

func (fdl *FakeDataLayer) CreateQAEnvironment(qa *QAEnvironment) error {
	fdl.data.Lock()
	fdl.data.d[qa.Name] = qa
	fdl.data.Unlock()
	return nil
}

func (fdl *FakeDataLayer) GetQAEnvironment(name string) (*QAEnvironment, error) {
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	if qa, ok := fdl.data.d[name]; ok {
		return qa, nil
	}
	return nil, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentConsistently(name string) (*QAEnvironment, error) {
	return fdl.GetQAEnvironment(name)
}

func (fdl *FakeDataLayer) GetQAEnvironments() ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		out = append(out, *v)
	}
	return out, nil
}

func (fdl *FakeDataLayer) DeleteQAEnvironment(name string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		if v.Status != models.Destroyed {
			return errors.New("status must be Destroyed")
		}
		delete(fdl.data.d, name)
	} else {
		return errors.New("env not found")
	}
	if _, ok := fdl.data.k8s[name]; ok {
		delete(fdl.data.k8s, name)
	}
	if _, ok := fdl.data.helm[name]; ok {
		delete(fdl.data.helm, name)
	}
	return nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsByStatus(status string) ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	s, err := models.EnvironmentStatusFromString(status)
	if err != nil {
		return nil, errors.Wrap(err, "bad status")
	}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.Status == s {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetRunningQAEnvironments() ([]QAEnvironment, error) {
	out1, _ := fdl.GetQAEnvironmentsByStatus("Success")
	out2, _ := fdl.GetQAEnvironmentsByStatus("Spawned")
	out3, _ := fdl.GetQAEnvironmentsByStatus("Updating")
	out := append(out1, append(out2, out3...)...)
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsByRepoAndPR(repo string, pr uint) ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.Repo == repo && v.PullRequest == pr {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsByRepo(repo string) ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.Repo == repo {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentBySourceSHA(sourceSHA string) (*QAEnvironment, error) {
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.SourceSHA == sourceSHA {
			return v, nil
		}
	}
	return nil, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsBySourceBranch(sourceBranch string) ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.SourceBranch == sourceBranch {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsByUser(user string) ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.User == user {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetExtantQAEnvironments(repo string, pr uint) ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.Repo == repo && v.PullRequest == pr && v.Status != models.Destroyed && v.Status != models.Failure {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) SetQAEnvironmentStatus(name string, status EnvironmentStatus) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.Status = status
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetQAEnvironmentRepoData(name string, rrd *RepoRevisionData) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.Repo = rrd.Repo
		v.PullRequest = rrd.PullRequest
		v.BaseSHA = rrd.BaseSHA
		v.BaseBranch = rrd.BaseBranch
		v.SourceRef = rrd.SourceRef
		v.SourceSHA = rrd.SourceSHA
		v.SourceBranch = rrd.SourceBranch
		v.User = rrd.User
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetQAEnvironmentRefMap(name string, rm RefMap) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.RefMap = rm
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetQAEnvironmentCommitSHAMap(name string, csm RefMap) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.CommitSHAMap = csm
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetQAEnvironmentCreated(name string, ts time.Time) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.Created = ts
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetAminoEnvironmentID(name string, did int) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.AminoEnvironmentID = did
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetAminoServiceToPort(name string, serviceToPort map[string]int64) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.AminoServiceToPort = serviceToPort
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetAminoKubernetesNamespace(name, namespace string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.AminoKubernetesNamespace = namespace
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) AddEvent(name string, msg string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		e := models.QAEnvironmentEvent{
			Timestamp: time.Now().UTC(),
			Message:   msg,
		}
		v.Events = append(v.Events, e)
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) getQAEnvironmentsBySourceRef(sourceref string) ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.SourceRef == sourceref {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) Search(opts models.EnvSearchParameters) ([]QAEnvironment, error) {
	if opts.Pr != 0 && opts.Repo == "" {
		return nil, fmt.Errorf("search by PR requires repo name")
	}
	if opts.TrackingRef != "" && opts.Repo == "" {
		return nil, fmt.Errorf("search by tracking ref requires repo name")
	}
	filter := func(envs []models.QAEnvironment, cf func(e models.QAEnvironment) bool) []models.QAEnvironment {
		pres := []models.QAEnvironment{}
		for _, e := range envs {
			if cf(e) {
				pres = append(pres, e)
			}
		}
		return pres
	}
	envs, _ := fdl.GetQAEnvironments()
	if opts.Pr != 0 {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.PullRequest == opts.Pr && e.Repo == opts.Repo })
	}
	if opts.Repo != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.Repo == opts.Repo })
	}
	if opts.SourceSHA != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.SourceSHA == opts.SourceSHA })
	}
	if opts.SourceBranch != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.SourceBranch == opts.SourceBranch })
	}
	if opts.User != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.User == opts.User })
	}
	if opts.Status != models.UnknownStatus {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.Status == opts.Status })
	}
	if opts.TrackingRef != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.SourceRef == opts.TrackingRef })
	}
	return envs, nil
}

func (fdl *FakeDataLayer) GetMostRecent(n uint) ([]QAEnvironment, error) {
	envs, _ := fdl.GetQAEnvironments()
	sort.Slice(envs, func(i int, j int) bool { return envs[i].Created.After(envs[j].Created) })
	if int(n) > len(envs) {
		return envs, nil
	}
	return envs[0:n], nil
}

func (fdl *FakeDataLayer) Close() error {
	return nil
}

func (fdl *FakeDataLayer) GetHelmReleasesForEnv(name string) ([]models.HelmRelease, error) {
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	v, ok := fdl.data.helm[name]
	if ok {
		return v, nil
	}
	return nil, nil
}

func (fdl *FakeDataLayer) UpdateHelmReleaseRevision(envname, release, revision string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	for i, r := range fdl.data.helm[envname] {
		if r.Release == release {
			fdl.data.helm[envname][i].RevisionSHA = revision
		}
	}
	return nil
}

func (fdl *FakeDataLayer) CreateHelmReleasesForEnv(releases []models.HelmRelease) error {
	if len(releases) == 0 {
		return errors.New("empty releases")
	}
	name := releases[0].EnvName
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if _, ok := fdl.data.d[name]; !ok {
		return errors.New("env missing")
	}
	fdl.data.helm[name] = releases
	return nil
}

func (fdl *FakeDataLayer) DeleteHelmReleasesForEnv(name string) (uint, error) {
	var n uint
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.helm[name]; ok {
		n = uint(len(v))
	} else {
		return 0, nil
	}
	delete(fdl.data.helm, name)
	return n, nil
}

func (fdl *FakeDataLayer) GetK8sEnv(name string) (*models.KubernetesEnvironment, error) {
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	env, ok := fdl.data.k8s[name]
	if !ok {
		return nil, nil
	}
	return env, nil
}

func (fdl *FakeDataLayer) GetK8sEnvsByNamespace(ns string) ([]models.KubernetesEnvironment, error) {
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	var out []models.KubernetesEnvironment
	for _, v := range fdl.data.k8s {
		if v.Namespace == ns {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) CreateK8sEnv(env *models.KubernetesEnvironment) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if env == nil || env.EnvName == "" {
		return errors.New("malformed env: nil or empty name")
	}
	if _, ok := fdl.data.d[env.EnvName]; !ok {
		return errors.New("env not found")
	}
	fdl.data.k8s[env.EnvName] = env
	return nil
}

func (fdl *FakeDataLayer) DeleteK8sEnv(name string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	delete(fdl.data.k8s, name)
	return nil
}

func (fdl *FakeDataLayer) UpdateK8sEnvTillerAddr(envname, taddr string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	env, ok := fdl.data.k8s[envname]
	if ok {
		env.TillerAddr = taddr
		fdl.data.k8s[envname] = env
	}
	return nil
}

func (fdl *FakeDataLayer) GetEventLogByID(id uuid.UUID) (*models.EventLog, error) {
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	el, ok := fdl.data.elogs[id]
	if !ok {
		return nil, nil
	}
	return el, nil
}

func (fdl *FakeDataLayer) GetEventLogsByEnvName(name string) ([]models.EventLog, error) {
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	var out []models.EventLog
	for k := range fdl.data.elogs {
		if fdl.data.elogs[k].EnvName == name {
			out = append(out, *fdl.data.elogs[k])
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetEventLogsByRepoAndPR(repo string, pr uint) ([]models.EventLog, error) {
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	var out []models.EventLog
	for k := range fdl.data.elogs {
		if fdl.data.elogs[k].Repo == repo && fdl.data.elogs[k].PullRequest == pr {
			out = append(out, *fdl.data.elogs[k])
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) CreateEventLog(elog *models.EventLog) error {
	if elog == nil {
		return errors.New("input is nil")
	}
	fdl.data.Lock()
	defer fdl.data.Unlock()
	fdl.data.elogs[elog.ID] = elog
	return nil
}

func (fdl *FakeDataLayer) AppendToEventLog(id uuid.UUID, msg string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if fdl.data.elogs[id] == nil {
		return errors.New("id not found")
	}
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, msg)
	return nil
}

func (fdl *FakeDataLayer) SetEventLogEnvName(id uuid.UUID, name string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	fdl.data.elogs[id].EnvName = name
	return nil
}

func (fdl *FakeDataLayer) DeleteEventLog(id uuid.UUID) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	delete(fdl.data.elogs, id)
	return nil
}

func (fdl *FakeDataLayer) DeleteEventLogsByEnvName(name string) (uint, error) {
	del := []uuid.UUID{}
	fdl.data.Lock()
	for k := range fdl.data.elogs {
		if fdl.data.elogs[k].EnvName == name {
			del = append(del, fdl.data.elogs[k].ID)
		}
	}
	for _, id := range del {
		delete(fdl.data.elogs, id)
	}
	fdl.data.Unlock()
	return uint(len(del)), nil
}

func (fdl *FakeDataLayer) DeleteEventLogsByRepoAndPR(repo string, pr uint) (uint, error) {
	del := []uuid.UUID{}
	fdl.data.Lock()
	for k := range fdl.data.elogs {
		if fdl.data.elogs[k].Repo == repo && fdl.data.elogs[k].PullRequest == pr {
			del = append(del, fdl.data.elogs[k].ID)
		}
	}
	for _, id := range del {
		delete(fdl.data.elogs, id)
	}
	fdl.data.Unlock()
	return uint(len(del)), nil
}

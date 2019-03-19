package spawner

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"golang.org/x/net/context"

	"github.com/dollarshaveclub/acyl/pkg/locker"
	"github.com/dollarshaveclub/acyl/pkg/spawner/pbaminoapi"
	furan "github.com/dollarshaveclub/furan/rpcclient"
	newrelic "github.com/newrelic/go-agent"
)

const (
	furanBuildTimeoutMins        = 30
	githubStatusContext          = "Acyl"
	dnsTTL                       = 60
	TagBranchMatchFallbackBranch = "master"
	nameGenMaxRetries            = 10
)

// EnvironmentSpawner describes an object capable of managing environments
type EnvironmentSpawner interface {
	Create(context.Context, RepoRevisionData) (string, error)
	Update(context.Context, RepoRevisionData) (string, error)
	Destroy(context.Context, RepoRevisionData, QADestroyReason) error
	DestroyExplicitly(context.Context, *QAEnvironment, QADestroyReason) error
	Success(context.Context, string) error
	Failure(context.Context, string, string) error
}

// DNSManager describes an object that can create or delete DNS records
type DNSManager interface {
	GetPrimaryHostname([]string, string) (string, error)
	CreateRecords(string, string, *QAType) error
	DeleteRecords(string, string, *QAType) error
}

type OperationMetricsCollector interface {
	Operation(op, name, repo, ref string, err error)
}

type ProvisioningMetricsCollector interface {
	ProvisioningMetricsTimer

	ProvisioningDuration(string, string, string, time.Duration, error)
	ContainerBuildAllDuration(string, string, string, time.Duration, error)
	ContainerBuildDuration(string, string, string, string, string, time.Duration, error)

	Success(string, string, string)
	Failure(string, string, string)
	AminoDeployTimedOut(name, repo, ref string)
	ImageBuildFailed(name, repo, ref string)
}

type ProvisioningMetricsTimer interface {
	TimeProvisioning(string, string, string, *error) func()
	TimeContainerBuildAll(string, string, string, *error) func()
	TimeContainerBuild(string, string, string, string, string, *error) func()
}

const (
	nrTxnContextKey = "newrelic_txn"
)

type AcylContextKey string // https://github.com/golang/lint/pull/245#issuecomment-255496398

// NewNRTxnContext returns a context with the New Relic transaction embedded as a value
func NewNRTxnContext(ctx context.Context, txn newrelic.Transaction) context.Context {
	return context.WithValue(ctx, AcylContextKey(nrTxnContextKey), txn)
}

// GetNRTxnFromContext returns the New Relic transaction (or nil) and a boolean indicating whether it exists
func GetNRTxnFromContext(ctx context.Context) (newrelic.Transaction, bool) {
	txn, ok := ctx.Value(AcylContextKey(nrTxnContextKey)).(newrelic.Transaction)
	return txn, ok
}

type refMatcher struct {
}

// getRefForOtherRepo contains the core branch matching algorithm. The input is the PR info and the branches for the repo in question.
// Output is the branch and SHA to use for the repo
func (matcher refMatcher) getRefForOtherRepo(rd *RepoRevisionData, override string, branches []BranchInfo) (string, string, error) {
	var branch, sha string
	var hasSourceBranch, hasBaseBranch, hasOverrideBranch bool

	// map of branch name -> SHA
	binfo := map[string]string{}
	for _, bi := range branches {
		binfo[bi.Name] = bi.SHA
	}

	if _, ok := binfo[rd.SourceBranch]; ok {
		hasSourceBranch = true
	}
	if _, ok := binfo[rd.BaseBranch]; ok {
		hasBaseBranch = true
	}
	if _, ok := binfo[override]; ok && override != "" {
		hasOverrideBranch = true
	}

	if !hasSourceBranch && !hasBaseBranch && !hasOverrideBranch {
		var fb, fl string
		if override != "" {
			fb = override
			fl = "BranchOverride"
		} else {
			fb = rd.BaseBranch
			fl = "BaseBranch"
		}
		return "", "", fmt.Errorf(`no suitable branch: neither "%v" (SourceBranch) nor "%v" (%v) found`, rd.SourceBranch, fb, fl)
	}

	// order determines precedence
	if hasBaseBranch {
		branch = rd.BaseBranch
	}
	if hasOverrideBranch {
		branch = override
	}
	if hasSourceBranch {
		branch = rd.SourceBranch
	}
	sha = binfo[branch]

	return sha, branch, nil
}

// QASpawner is an object that can manage environments in AWS
type QASpawner struct {
	dl                DataLayer
	ng                NameGenerator
	rc                RepoClient
	lp                PreemptiveLockProvider
	fa                []string
	caddr             string
	cn                ChatNotifier
	pmc               ProvisioningMetricsCollector
	omc               OperationMetricsCollector
	nrapp             newrelic.Application
	logger            *log.Logger
	aminoBackend      AcylBackend
	aminoConfig       *AminoConfig
	typepath          string
	globalLimit       uint
	hostnameTemplate  string
	furanClientDDName string
}

// NewQASpawner returns a new QASpawner instance with the specified logger and datalayer
func NewQASpawner(logger *log.Logger, dl DataLayer, ng NameGenerator, rc RepoClient, lp PreemptiveLockProvider, furanAddrs []string, consulAddr string, cn ChatNotifier, omc MetricsCollector, pmc MetricsCollector, nrapp newrelic.Application, awsCreds *AWSCreds, awsConfig *AWSConfig, backendConfig *BackendConfig, ac *AminoConfig, typepath string, globalLimit uint, hostnameTemplate string, furanClientDDName string) (*QASpawner, error) {
	qs := &QASpawner{
		logger:            logger,
		dl:                dl,
		ng:                ng,
		rc:                rc,
		lp:                lp,
		fa:                furanAddrs,
		caddr:             consulAddr,
		cn:                cn,
		pmc:               pmc,
		omc:               omc,
		nrapp:             nrapp,
		typepath:          typepath,
		globalLimit:       globalLimit,
		hostnameTemplate:  hostnameTemplate,
		furanClientDDName: furanClientDDName,
	}
	conn, err := grpc.Dial(backendConfig.AminoAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	qs.aminoBackend = &AminoBackend{
		aminoClient: pbaminoapi.NewAminoAPIClient(conn),
		aminoConfig: ac,
		logger:      qs.logger,
		dataLayer:   dl,
		spawner:     qs,
		metrics:     pmc,
	}

	return qs, nil
}

type branchresult struct {
	repo   string
	err    error
	branch string
	tag    string
	sha    string
}

// getTagMap determines the tags to use for other repos, if any
// if a matching tag isn't found, fall back to TagBranchMatchFallbackBranch
func (qs *QASpawner) getTagMap(qat *QAType, rd *RepoRevisionData) (map[string]branchresult, error) {
	if rd.SourceBranch != "" {
		return nil, fmt.Errorf("not a tracking environment")
	}

	output := map[string]branchresult{}
	rchan := make(chan branchresult, len(qat.OtherRepos))

	output[qat.TargetRepo] = branchresult{
		tag: rd.SourceRef,
		sha: rd.SourceSHA,
	}

	for _, r := range qat.OtherRepos {
		go func(otherRepo string) {
			var b BranchInfo
			override := TagBranchMatchFallbackBranch
			if v, ok := qat.BranchOverrides[otherRepo]; ok {
				override = v
			}

			result := branchresult{
				repo: otherRepo,
			}

			otherReposTags, err := qs.rc.GetTags(context.Background(), otherRepo)
			if err != nil {
				result.err = err
				goto done
			}

			for _, t := range otherReposTags {
				if "tags/"+t.Name == rd.SourceRef { // SourceRef contains the prefix "tags/" to disambiguate from branch names
					result.tag = rd.SourceRef
					result.sha = t.SHA
					goto done
				}
			}

			b, err = qs.rc.GetBranch(context.Background(), otherRepo, override)
			if err != nil {
				result.err = err
				goto done
			}

			result.branch = override
			result.sha = b.SHA

		done:

			select {
			case rchan <- result:
				return
			default:
				qs.logger.Printf("getTagMap: would have blocked! returning")
				return // should never happen
			}
		}(r)
	}

	for _ = range qat.OtherRepos {
		res := <-rchan
		if res.err != nil {
			return nil, fmt.Errorf("error calculating tag/sha for repo(%s): %v", qat.TargetRepo, res.err)
		}
		output[res.repo] = res
	}
	return output, nil
}

// getRefMap makes GitHub API calls to determine the ref (branch/sha) each repo should use
// within the new environment.
// Rules:
//  - If SourceBranch exists, use it for TargetRepo and all OtherRepos
//  - If SourceBranch doesn't exist on TargetRepo, it's a fatal error (note: it should be impossible to open a PR from a branch that doesn't exist)
//  - If SourceBranch doesn't exist on an OtherRepo, fall back to BaseBranch
//  - If BaseBranch doesn't exist on OtherRepo, it's a fatal error (entirely possible)
func (qs *QASpawner) getRefMap(qat *QAType, rd *RepoRevisionData) (map[string]branchresult, error) {
	if rd.SourceBranch == "" && strings.HasPrefix(rd.SourceRef, "tags/") {
		return qs.getTagMap(qat, rd)
	}

	var found bool
	output := map[string]branchresult{}
	targetBranches, err := qs.rc.GetBranches(context.Background(), qat.TargetRepo)

	if err != nil {
		return nil, fmt.Errorf("error getting branches for TargetRepo (%v): %v", qat.TargetRepo, err)
	}

	for _, b := range targetBranches {
		if b.Name == rd.SourceBranch {
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("SourceBranch (%v) not found in TargetRepo (%v)", rd.SourceBranch, qat.TargetRepo)
	}

	output[qat.TargetRepo] = branchresult{
		repo:   qat.TargetRepo,
		branch: rd.SourceBranch,
		sha:    rd.SourceSHA,
	}

	// Calculate branches for OtherRepos in parallel
	matcher := refMatcher{}
	rchan := make(chan branchresult, len(qat.OtherRepos))

	for _, r := range qat.OtherRepos {
		go func(otherRepo string) {
			var override, sha, branch string
			var err error
			if v, ok := qat.BranchOverrides[otherRepo]; ok {
				override = v
			}

			result := branchresult{
				repo: otherRepo,
			}

			otherReposBranches, err := qs.rc.GetBranches(context.Background(), otherRepo)

			if err != nil {
				result.err = err
				goto done
			}

			sha, branch, err = matcher.getRefForOtherRepo(rd, override, otherReposBranches)
			if err != nil {
				result.err = err
				goto done
			}

			result.branch = branch
			result.sha = sha

		done:

			select {
			case rchan <- result:
				return
			default:
				qs.logger.Printf("getRefMap: would have blocked! returning")
				return // should never happen
			}
		}(r)
	}

	for _ = range qat.OtherRepos {
		res := <-rchan
		if res.err != nil {
			return nil, fmt.Errorf("error calculating branch for repo(%s): %v", qat.TargetRepo, res.err)
		}
		output[res.repo] = res
	}
	return output, nil
}

// buildContainer triggers Furan to build repo at ref, writing to done when the
// build completes
func (qs QASpawner) buildContainer(ctx context.Context, rd *RepoRevisionData, name string, repo string, ref string, done chan error) {
	fcopts := &furan.DiscoveryOptions{}
	if len(qs.fa) > 0 {
		fcopts.NodeList = qs.fa
	} else {
		fcopts.UseConsul = true
		fcopts.ConsulAddr = qs.caddr
		fcopts.SelectionStrategy = furan.RandomNodeSelection
		fcopts.ServiceName = "furan"
	}
	fc, err := furan.NewFuranClient(fcopts, qs.logger, qs.furanClientDDName)
	if err != nil {
		done <- err
	}
	req := furan.BuildRequest{
		Build: &furan.BuildDefinition{
			GithubRepo: repo,
			Ref:        ref,
			Tags:       []string{ref},
			Args:       map[string]string{"GIT_COMMIT_SHA": ref},
		},
		Push: &furan.PushDefinition{
			Registry: &furan.PushRegistryDefinition{
				Repo: fmt.Sprintf("quay.io/%v", repo),
			},
			S3: &furan.PushS3Definition{},
		},
		SkipIfExists: true,
	}
	bchan := make(chan *furan.BuildEvent)
	go func() {
		var build, push bool
		for event := range bchan {
			if event.EventType == furan.BuildEvent_DOCKER_BUILD_STREAM && !build {
				qs.logger.Printf("furan: %v: building (build id: %v)", repo, event.BuildId)
				build = true
			}
			if event.EventType == furan.BuildEvent_DOCKER_PUSH_STREAM && !push {
				qs.logger.Printf("furan: %v: pushing (build id: %v)", repo, event.BuildId)
				push = true
			}
		}
	}()
	qs.dl.AddEvent(name, fmt.Sprintf("building container: %v:%v", repo, ref))

	defer qs.pmc.TimeContainerBuild(name, rd.Repo, rd.SourceBranch, repo, ref, &err)()

	retries := 3
	var buildErr error
	for i := 0; i < retries; i++ {
		id, err := fc.Build(ctx, bchan, &req)
		if err != nil {

			if err == furan.ErrCanceled {
				break // suppress error
			}

			buildErr = err
			errmsg := fmt.Sprintf("build failed: %v: %v: %v", repo, id, err)
			qs.logger.Printf(errmsg)
			qs.dl.AddEvent(name, errmsg)

			if i != retries-1 {
				qs.dl.AddEvent(name, fmt.Sprintf("retrying image build: %v", repo))
			}

			continue
		}

		okmsg := fmt.Sprintf("build finished: %v: %v", repo, id)
		qs.logger.Printf(okmsg)
		qs.dl.AddEvent(name, okmsg)
		buildErr = nil
		break
	}
	close(bchan)

	if buildErr != nil {
		buildErr = fmt.Errorf("%v: %v", repo, buildErr)
	}
	select {
	case done <- buildErr:
		return
	default:
		qs.logger.Printf("buildContainer: would have blocked! returning")
	}
}

// buildAllContainers triggers Furan to build all required containers in parallel
func (qs QASpawner) buildAllContainers(ctx context.Context, name string, rd *RepoRevisionData, rm map[string]string) (result error) {
	done := make(chan error, len(rm))
	cancelfuncs := []context.CancelFunc{}

	defer qs.pmc.TimeContainerBuildAll(name, rd.Repo, rd.SourceBranch, &result)()

	for repo, sha := range rm {
		ctx, cf := context.WithTimeout(context.Background(), furanBuildTimeoutMins*time.Minute)
		cancelfuncs = append(cancelfuncs, cf)
		go qs.buildContainer(ctx, rd, name, repo, sha, done)
	}
	var failed bool
	errs := []error{}
	for _ = range rm {
		select {
		case err := <-done:
			if err != nil { // if any builds fail, cancel all builds
				failed = true
				errs = append(errs, err)
				for _, cancelFunc := range cancelfuncs {
					cancelFunc()
				}
			}
		case <-ctx.Done():
			for _, cancelFunc := range cancelfuncs {
				cancelFunc()
			}
			return ctx.Err()
		}
	}
	if failed {
		errsl := []string{}
		for _, e := range errs {
			if e != nil {
				errsl = append(errsl, e.Error())
			}

			go qs.pmc.ImageBuildFailed(name, rd.Repo, rd.SourceBranch)
		}
		return fmt.Errorf("container build(s) failed: %v", strings.Join(errsl, ", "))
	}
	return nil
}

func (qs *QASpawner) getHostname(qaName string) (string, error) {
	var b bytes.Buffer
	t, err := template.New("").Parse(qs.hostnameTemplate)
	if err != nil {
		return "", err
	}
	if err := t.Execute(&b, struct {
		Name string
	}{qaName}); err != nil {
		return "", err
	}
	return b.String(), nil
}

// persistQARecord writes a new QA record to the database
func (qs *QASpawner) persistQARecord(name string, qat *QAType, rd *RepoRevisionData, rm, cm map[string]string) error {
	if rd == nil {
		return fmt.Errorf("RepoRevisionData is nil")
	}
	if qat == nil {
		return fmt.Errorf("QAType is nil")
	}

	hostname, err := qs.getHostname(name)
	if err != nil {
		return fmt.Errorf("error generating hostname: %v", err)
	}

	qae := &QAEnvironment{
		Name:         name,
		Hostname:     hostname,
		User:         rd.User,
		Repo:         rd.Repo,
		PullRequest:  rd.PullRequest,
		SourceSHA:    rd.SourceSHA,
		SourceBranch: rd.SourceBranch,
		BaseSHA:      rd.BaseSHA,
		BaseBranch:   rd.BaseBranch,
		Created:      time.Now().UTC(),
		QAType:       qat.Name,
		Status:       Spawned,
		RefMap:       rm,
		CommitSHAMap: cm,
	}
	return qs.dl.CreateQAEnvironment(qae)
}

func (qs *QASpawner) createSpawningGithubStatus(rd *RepoRevisionData) error {
	cs := &CommitStatus{
		Context:     githubStatusContext,
		Status:      "pending",
		Description: "Dynamic QA is being created",
		TargetURL:   "https://media.giphy.com/media/oiymhxu13VYEo/giphy.gif",
	}
	return qs.rc.SetStatus(context.Background(), rd.Repo, rd.SourceSHA, cs)
}

func (qs *QASpawner) createBuildingContainersGithubStatus(rd *RepoRevisionData, rm map[string]string) error {
	cs := &CommitStatus{
		Context:     githubStatusContext,
		Status:      "pending",
		Description: fmt.Sprintf("Container images building (%v)", len(rm)),
		TargetURL:   "https://media.giphy.com/media/oiymhxu13VYEo/giphy.gif",
	}
	return qs.rc.SetStatus(context.Background(), rd.Repo, rd.SourceSHA, cs)
}

func (qs *QASpawner) createErrorGithubStatus(rd *RepoRevisionData, msg string) error {
	cs := &CommitStatus{
		Context:     githubStatusContext,
		Status:      "error",
		Description: fmt.Sprintf("Error creating dynamic QA: %v", msg),
		TargetURL:   "https://media.giphy.com/media/pyFsc5uv5WPXN9Ocki/giphy.gif",
	}
	return qs.rc.SetStatus(context.Background(), rd.Repo, rd.SourceSHA, cs)
}

func (qs *QASpawner) getType(repo string, ref string) (*QAType, error) {
	tb, err := qs.rc.GetFileContents(context.Background(), repo, qs.typepath, ref)
	if err != nil {
		return nil, fmt.Errorf("error fetching type from repo: %v: %v", repo, err)
	}
	qat := QAType{}
	err = qat.FromYAML(tb)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling type from repo: %v: %v", repo, err)
	}
	qat.TargetRepo = repo
	return &qat, nil
}

// Create creates a new QA of the specified type, returning the name
func (qs *QASpawner) Create(ctx context.Context, rd RepoRevisionData) (string, error) {
	txn := qs.nrapp.StartTransaction("AsyncCreate", nil, nil)
	defer txn.End()
	ctx = NewNRTxnContext(ctx, txn)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lock := qs.lp.NewPreemptiveLocker(rd.Repo, strconv.Itoa(int(rd.PullRequest)), locker.PreemptiveLockerOpts{})
	rch, err := lock.Lock(ctx)
	if err != nil {
		err = fmt.Errorf("error acquiring lock: %v", err)
		qs.logger.Printf("%v", err)
		txn.NoticeError(err)
		return "", err
	}

	clCh := make(chan struct{})
	defer close(clCh)
	go func() {
		select {
		case <-rch: // Lock got preempted, cancel action
			cancel()
		case <-clCh:
			cancel()
		}
	}()

	defer lock.Release()
	name, err := qs.create(ctx, rd, false)
	if err != nil {
		txn.NoticeError(err)
		return name, err
	}
	return name, nil
}

// Update destroys and replaces a QA environment with new code (if one exists) but same name
func (qs *QASpawner) Update(ctx context.Context, rd RepoRevisionData) (string, error) {
	txn := qs.nrapp.StartTransaction("AsyncUpdate", nil, nil)
	defer txn.End()
	ctx = NewNRTxnContext(ctx, txn)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lock := qs.lp.NewPreemptiveLocker(rd.Repo, strconv.Itoa(int(rd.PullRequest)), locker.PreemptiveLockerOpts{})
	rch, err := lock.Lock(ctx)
	if err != nil {
		err = fmt.Errorf("error acquiring lock: %v", err)
		qs.logger.Printf("%v", err)
		txn.NoticeError(err)
		return "", err
	}

	clCh := make(chan struct{})
	defer close(clCh)
	go func() {
		select {
		case <-rch: // Lock got preempted, cancel action
			cancel()
		case <-clCh:
			cancel()
		}
	}()

	defer lock.Release()
	name, err := qs.update(ctx, rd)
	if err != nil {
		txn.NoticeError(err)
		return name, err
	}
	return name, nil
}

// Destroy destroys a specific named QA environment
func (qs *QASpawner) Destroy(ctx context.Context, rd RepoRevisionData, reason QADestroyReason) error {
	txn := qs.nrapp.StartTransaction("AsyncDestroy", nil, nil)
	defer txn.End()
	ctx = NewNRTxnContext(ctx, txn)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lock := qs.lp.NewPreemptiveLocker(rd.Repo, strconv.Itoa(int(rd.PullRequest)), locker.PreemptiveLockerOpts{})
	rch, err := lock.Lock(ctx)
	if err != nil {
		err = fmt.Errorf("error acquiring lock: %v", err)
		qs.logger.Printf("%v", err)
		txn.NoticeError(err)
		return err
	}

	clCh := make(chan struct{})
	defer close(clCh)
	go func() {
		select {
		case <-rch: // Lock got preempted, cancel action
			cancel()
		case <-clCh:
			cancel()
		}
	}()

	defer lock.Release()
	_, err = qs.destroy(ctx, rd, reason, true)
	if err != nil {
		txn.NoticeError(err)
		return err
	}
	return nil
}

// isDestroyed checks if the given QA environment has been marked as destroyed
func (qs *QASpawner) isDestroyed(name string) bool {
	qae, _ := qs.dl.GetQAEnvironmentConsistently(name)
	return qae == nil || qae.Status == Destroyed
}

// destroyStale destroys all but the latest in a slice of environments, returning the single environment left
func (qs *QASpawner) destroyStale(ctx context.Context, qas []QAEnvironment) (*QAEnvironment, error) {
	var qaes QAEnvironments
	if len(qas) < 2 {
		return nil, fmt.Errorf("destroyStale: need >= 2 environments (received %v)", len(qas))
	}
	qaes = qas
	sort.Sort(qaes)
	var err error
	for i, e := range qaes {
		if i != len(qaes)-1 {
			err = qs.destroyQA(ctx, &e)
			if err != nil {
				return nil, fmt.Errorf("error destroying extant QA: %v", err)
			}
		}
	}
	return &qaes[len(qaes)-1], nil
}

// Check running instances against configured global limit against running environments + new requests (n)
// If necessary, kill oldest environments to avoid going over global limit
func (qs *QASpawner) checkGlobalLimit(ctx context.Context, n uint) error {
	if qs.globalLimit == 0 {
		return nil
	}
	qae, err := qs.dl.GetRunningQAEnvironments()
	if err != nil {
		return fmt.Errorf("error getting running environments: %v", err)
	}
	if len(qae)+int(n) > int(qs.globalLimit) {
		kc := (len(qae) + int(n)) - int(qs.globalLimit)
		sort.Slice(qae, func(i int, j int) bool { return qae[i].Created.Before(qae[j].Created) })
		kenvs := qae[0:kc]
		qs.logger.Printf("spawner: enforcing global limit: extant: %v, limit: %v, destroying: %v", len(qae), qs.globalLimit, kc)
		for _, e := range kenvs {
			env := e
			qs.logger.Printf("spawner: destroying: %v (created %v)", env.Name, env.Created)
			err := qs.DestroyExplicitly(ctx, &env, EnvironmentLimitExceeded)
			if err != nil {
				qs.logger.Printf("error destroying environment for exceeding limit: %v", err)
			}
		}
	} else {
		qs.logger.Printf("global limit not exceeded: running: %v, limit: %v", len(qae)+1, qs.globalLimit)
	}
	return nil
}

func (qs *QASpawner) getMetadata(ctx context.Context, rd RepoRevisionData) (commitMap map[string]string, refMap map[string]string, qat *QAType, rr error) {
	txn, ok := GetNRTxnFromContext(ctx)
	if !ok {
		return nil, nil, nil, fmt.Errorf("context is missing New Relic transaction")
	}

	nrseg := newrelic.StartSegment(txn, "createGetMetadata")
	defer nrseg.End()

	err := qs.checkGlobalLimit(ctx, 1)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error enforcing global limit (%v): %v", qs.globalLimit, err)
	}

	qat, err = qs.getType(rd.Repo, rd.SourceSHA)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting QAType: %v", err)
	}

	brm, err := qs.getRefMap(qat, &rd)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error generating ref map: %v", err)
	}

	rm := map[string]string{} // RefMap
	cm := map[string]string{} // CommitSHAMap
	for k, br := range brm {
		if br.branch == "" {
			rm[k] = br.tag
		} else {
			rm[k] = br.branch
		}
		cm[k] = br.sha
	}
	return cm, rm, qat, nil
}

// updateQAMetadata updates the event metadata for an existing environment and returns the updated structure
func (qs *QASpawner) updateQAMetadata(name string, status EnvironmentStatus, rd RepoRevisionData, cm map[string]string, rm map[string]string) (*QAEnvironment, error) {
	err := qs.dl.SetQAEnvironmentRepoData(name, &rd)
	if err != nil {
		return nil, fmt.Errorf("error setting RepoData: %v: %v", name, err)
	}
	err = qs.dl.SetQAEnvironmentCommitSHAMap(name, cm)
	if err != nil {
		return nil, fmt.Errorf("error setting commit SHA map: %v: %v", name, err)
	}
	err = qs.dl.SetQAEnvironmentRefMap(name, rm)
	if err != nil {
		return nil, fmt.Errorf("error setting ref map: %v: %v", name, err)
	}
	err = qs.dl.SetQAEnvironmentCreated(name, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("error setting created: %v: %v", name, err)
	}
	err = qs.dl.SetQAEnvironmentStatus(name, status)
	if err != nil {
		return nil, fmt.Errorf("error setting status: %v: %v", name, err)
	}
	qa, err := qs.dl.GetQAEnvironmentConsistently(name)
	if err != nil {
		return nil, fmt.Errorf("error getting QA environment: %v: %v", name, err)
	}
	return qa, nil
}

func (qs *QASpawner) genUniqueName() (string, error) {
	for i := 0; i < nameGenMaxRetries; i++ {
		name, err := qs.ng.New()
		if err != nil {
			return "", fmt.Errorf("error generating name: %v", err)
		}
		qa, err := qs.dl.GetQAEnvironmentConsistently(name)
		if err != nil {
			return "", fmt.Errorf("error getting QA environment: %v", err)
		}
		if qa == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("max retries exceeded (%v) attempting to generate unique name", nameGenMaxRetries)
}

// ensureOne guarantees that this event has exactly one extant DB record with updated metadata
func (qs *QASpawner) ensureOne(ctx context.Context, status EnvironmentStatus, rd RepoRevisionData, qt *QAType, cm map[string]string, rm map[string]string) (qa *QAEnvironment, err error) {
	txn, ok := GetNRTxnFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("context is missing New Relic transaction")
	}
	nrseg := newrelic.StartSegment(txn, "createEnsureOne")
	defer nrseg.End()

	qas, err := qs.dl.GetQAEnvironmentsByRepoAndPR(rd.Repo, rd.PullRequest)
	if err != nil {
		return nil, fmt.Errorf("error getting environments by repo and PR: %v", err)
	}
	// if PR == 0 (not attached to PR), filter out by the source ref for this event
	if rd.PullRequest == 0 {
		filtered := []QAEnvironment{}
		for _, qa := range qas {
			if qa.SourceRef == rd.SourceRef {
				filtered = append(filtered, qa)
			}
		}
		qas = filtered
	}
	if len(qas) == 0 { // none existing, create a new name and record
		name, err := qs.genUniqueName()
		if err != nil {
			return nil, fmt.Errorf("error generating name: %v", err)
		}
		qt.Name = name
		err = qs.persistQARecord(name, qt, &rd, rm, cm)
		if err != nil {
			return nil, fmt.Errorf("error persisting QA record: %v", err)
		}
		qa, err = qs.dl.GetQAEnvironmentConsistently(name)
		if err != nil {
			return nil, fmt.Errorf("error getting new QA record: %v", err)
		}
		return qa, nil
	}
	sort.Slice(qas, func(i int, j int) bool { return qas[i].Created.Before(qas[j].Created) })
	if len(qas) > 1 {
		_, err = qs.destroyStale(ctx, qas)
		if err != nil {
			return nil, fmt.Errorf("error destroying stale environments: %v", err)
		}
	}
	qa = &qas[len(qas)-1]
	qa, err = qs.updateQAMetadata(qa.Name, status, rd, cm, rm)
	if err != nil {
		return nil, fmt.Errorf("error updating metadata: %v", err)
	}
	return qa, nil
}

func (qs *QASpawner) chatNotifications(ctx context.Context, name string, rd RepoRevisionData, updating bool) {
	txn, ok := GetNRTxnFromContext(ctx)
	if !ok {
		qs.logger.Printf("chatNotifications: context is missing New Relic transaction")
		return
	}
	nrseg := newrelic.StartSegment(txn, "createChatNotifications")
	defer nrseg.End()

	var err error
	var cmsg string
	if updating {
		cmsg, err = qs.rc.GetCommitMessage(context.Background(), rd.Repo, rd.SourceSHA)
		if err != nil {
			qs.logger.Printf("error getting commit message: %v", err)
		}
		err = qs.cn.Updating(name, &rd, cmsg)
		if err != nil {
			qs.logger.Printf("error pushing chat Updating notification: %v", err)
			txn.NoticeError(err)
		}
	} else {
		err = qs.cn.Creating(name, &rd)
		if err != nil {
			qs.logger.Printf("error pushing chat Creating notification: %v", err)
			txn.NoticeError(err)
		}
	}
}

func (qs *QASpawner) buildImages(ctx context.Context, qa *QAEnvironment) error {
	txn, ok := GetNRTxnFromContext(ctx)
	if !ok {
		return fmt.Errorf("context is missing New Relic transaction")
	}
	nrseg := newrelic.StartSegment(txn, "createBuildImages")
	defer nrseg.End()

	rd := qa.RepoRevisionDataFromQA()
	err := qs.createBuildingContainersGithubStatus(rd, qa.RefMap)
	if err != nil {
		qs.logger.Printf("error creating GitHub building status: %v", err)
	}

	return qs.buildAllContainers(ctx, qa.Name, rd, qa.CommitSHAMap)
}

func (qs *QASpawner) aminoCreate(ctx context.Context, rd RepoRevisionData, qa *QAEnvironment, qat *QAType) (string, error) {
	txn, ok := GetNRTxnFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("context is missing New Relic transaction")
	}
	nrseg := newrelic.StartSegment(txn, "createAminoCreateEnvironment")
	defer nrseg.End()

	err := qs.createSpawningGithubStatus(&rd)
	if err != nil {
		qs.logger.Printf("error creating GitHub spawning status: %v", err)
	}
	return qs.aminoBackend.CreateEnvironment(ctx, qa, qat)
}

func (qs *QASpawner) create(ctx context.Context, rd RepoRevisionData, updating bool) (qaName string, resultError error) {
	/*
		-	gather/calculate metadata
		- ensure exactly one env for this event (terminate stale, create if needed, return name/info)
		- update database
		- chat notifications
		- perform creation via Amino
	*/
	var err error
	var name string
	defer func() { qs.omc.Operation("create", name, rd.Repo, rd.SourceBranch, resultError) }()
	defer qs.pmc.TimeProvisioning(name, rd.Repo, rd.SourceBranch, &resultError)()
	defer func() {
		if err != nil {
			go qs.pmc.Failure(name, rd.Repo, rd.SourceBranch)
			err2 := qs.cn.Failure(name, &rd, err.Error())
			if err2 != nil {
				qs.logger.Printf("error pushing chat Failure notification: %v", err2)
				txn, ok := GetNRTxnFromContext(ctx)
				if ok {
					txn.NoticeError(err2)
				}
			}
			err2 = qs.createErrorGithubStatus(&rd, err.Error())
			if err2 != nil {
				qs.logger.Printf("error setting Github failure status: %v", err2)
			}
			if name != "" && !qs.isDestroyed(name) {
				err2 = qs.dl.SetQAEnvironmentStatus(name, Failure)
				if err2 != nil {
					qs.logger.Printf("error setting environment to failed: %v", err2)
				}
			}
		}
	}()

	var qa *QAEnvironment
	var qat *QAType
	var cm, rm RefMap

	status := Spawned
	if updating {
		status = Updating
	}

	cm, rm, qat, err = qs.getMetadata(ctx, rd)
	if err != nil {
		return "", fmt.Errorf("error getting metadata: %v", err)
	}

	qa, err = qs.ensureOne(ctx, status, rd, qat, cm, rm)
	if err != nil {
		return "", fmt.Errorf("error ensuring one extant environment for event: %v", err)
	}

	name = qa.Name

	if ctx.Err() != nil { // We were cancelled
		return
	}

	qs.chatNotifications(ctx, name, rd, updating)

	err = qs.buildImages(ctx, qa)
	if err != nil {
		if grpc.Code(err) == codes.Canceled || err == context.Canceled { // Action cancelled
			qs.logger.Printf("build all containers canceled")
			err = nil        // Prevent the defer block from thinking an error happened
			return name, nil // Pretend like it succeeded
		}
		return name, fmt.Errorf("error building images: %v", err)
	}

	if ctx.Err() != nil { // We were cancelled
		return name, nil // Pretend like it succeeded
	}

	did, err := qs.aminoCreate(ctx, rd, qa, qat)
	if err != nil {
		if grpc.Code(err) == codes.Canceled || err == context.Canceled { // Action cancelled
			qs.logger.Printf("create environment canceled")
			err = nil        // Prevent the defer block from thinking an error happened
			return name, nil // Pretend like it succeeded
		}
		return name, fmt.Errorf("error creating environment: %v", err)
	}

	qs.logger.Printf("created environment: %v", did)

	return name, nil
}

func (qs *QASpawner) update(ctx context.Context, rd RepoRevisionData) (newName string, resultError error) {
	defer func() { qs.omc.Operation("replace", "", rd.Repo, rd.BaseBranch, resultError) }()
	return qs.create(ctx, rd, true)
}

func (qs *QASpawner) destroyQA(ctx context.Context, qae *QAEnvironment) error {
	if err := qs.retryPrintingErrors(ctx, "amino-environment-destroy", 6, func() error {
		err := qs.aminoBackend.DestroyEnvironment(ctx, qae, true)
		errorDesc := grpc.ErrorDesc(err)
		if errorDesc == "environment already destroyed" || errorDesc == "ID and name missing" {
			qs.logger.Printf("Skipping Retry of QASpawner.destroyQA for qae.Name: %v because aminoBackend.DestroyEnvironment error is %v\n", qae.Name, errorDesc)
			return nil
		}
		return err
	}); err != nil {
		qs.logger.Printf("error terminating Amino environment: %v", err)
	}

	return qs.dl.SetQAEnvironmentStatus(qae.Name, Destroyed)
}

func (qs *QASpawner) destroy(ctx context.Context, rd RepoRevisionData, reason QADestroyReason, notify bool) (result []string, resultError error) {
	defer func() { qs.omc.Operation("destroy", "", rd.Repo, rd.BaseBranch, resultError) }()
	qas, err := qs.dl.GetExtantQAEnvironments(rd.Repo, rd.PullRequest)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, qae := range qas {
		qae, err := qs.dl.GetQAEnvironmentConsistently(qae.Name)
		if err != nil {
			return names, fmt.Errorf("error getting QA environment: %v", err)
		}
		if qae.Status != Destroyed && qae.PullRequest == rd.PullRequest {
			err = qs.destroyQA(ctx, qae)
			if err != nil {
				msg := fmt.Errorf("error destroying QA: %v: %v", qae.Name, err)
				qs.cn.Failure(qae.Name, &rd, msg.Error())
				return names, msg
			}
			if notify {
				err = qs.cn.Destroying(qae.Name, &rd, reason)
				if err != nil {
					qs.logger.Printf("error pushing chat Destroying notification: %v", err)
					txn, ok := GetNRTxnFromContext(ctx)
					if ok {
						txn.NoticeError(err)
					}
				}
			}
			names = append(names, qae.Name)
		}
	}
	return names, nil
}

// DestroyExplicitly destroys a QA by name explicitly
func (qs *QASpawner) DestroyExplicitly(ctx context.Context, qa *QAEnvironment, reason QADestroyReason) error {
	txn := qs.nrapp.StartTransaction("AsyncDestroyExplicitly", nil, nil)
	defer txn.End()
	ctx = NewNRTxnContext(ctx, txn)

	err := qs.cn.Destroying(qa.Name, qa.RepoRevisionDataFromQA(), reason)
	if err != nil {
		err = fmt.Errorf("error pushing chat Destroying notification: %v", err)
		txn.NoticeError(err)
		qs.logger.Printf(err.Error())
	}
	err = qs.destroyQA(ctx, qa)
	if err != nil {
		txn.NoticeError(err)
		return err
	}
	return nil
}

func (qs *QASpawner) finalize(ctx context.Context, name string, state EnvironmentStatus, status string, desc string, failureMessage string) error {
	err := qs.dl.SetQAEnvironmentStatus(name, state)
	if err != nil {
		return err
	}
	err = qs.dl.AddEvent(name, fmt.Sprintf("marked as %v", state.String()))
	if err != nil {
		qs.logger.Printf("error adding event: %v: %v", name, err)
	}
	qa, err := qs.dl.GetQAEnvironment(name)
	if err != nil {
		return err
	}
	if state == Success {
		qs.pmc.Success(name, qa.Repo, qa.SourceBranch)
		err = qs.cn.Success(qa.Name, fmt.Sprintf("https://%v", qa.Hostname), qa.AminoServiceToPort, qa.AminoKubernetesNamespace, qa.RepoRevisionDataFromQA())
	} else {
		qs.pmc.Failure(name, qa.Repo, qa.SourceBranch)
		message := strings.Join([]string{"QA Environment deploy timed out:", failureMessage}, "\n")
		err = qs.cn.Failure(qa.Name, qa.RepoRevisionDataFromQA(), message)
	}
	if err != nil {
		qs.logger.Printf("error pushing chat notification: %v", err)
		txn, ok := GetNRTxnFromContext(ctx)
		if ok {
			txn.NoticeError(err)
		}
	}
	cs := &CommitStatus{
		Context:     githubStatusContext,
		Status:      status,
		Description: desc,
		TargetURL:   fmt.Sprintf("https://%v", qa.Hostname),
	}
	return qs.rc.SetStatus(context.Background(), qa.Repo, qa.SourceSHA, cs)
}

// Success marks an environment as successfully up and ready
func (qs *QASpawner) Success(ctx context.Context, name string) error {
	txn := qs.nrapp.StartTransaction("AsyncSuccess", nil, nil)
	defer txn.End()
	ctx = NewNRTxnContext(ctx, txn)

	err := qs.finalize(ctx, name, Success, "success", "QA is ready", "")
	if err != nil {
		txn.NoticeError(err)
		return err
	}
	return nil
}

// Failure marks an environment as failed
func (qs *QASpawner) Failure(ctx context.Context, name string, message string) error {
	txn := qs.nrapp.StartTransaction("AsyncFailure", nil, nil)
	defer txn.End()
	ctx = NewNRTxnContext(ctx, txn)

	err := qs.finalize(ctx, name, Failure, "failure", "(╯°□°）╯︵ ┻━┻ QA Failed", message)
	if err != nil {
		txn.NoticeError(err)
		return err
	}
	return nil
}

func (qs *QASpawner) retryPrintingErrors(ctx context.Context, name string, retries uint, f func() error) error {
	var err error
	for i := uint(0); i < retries; i++ {
		if err = f(); err != nil {
			qs.logger.Printf("error performing '%s', retrying: %v", name, err)
		} else {
			return nil
		}
		if i != retries-1 {
			time.Sleep(time.Second << i)
		}
	}
	return err
}

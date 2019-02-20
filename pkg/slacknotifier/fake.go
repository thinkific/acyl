package slacknotifier

type FakeChatNotifier struct{}

var _ ChatNotifier = &FakeChatNotifier{}

func (fcn FakeChatNotifier) Creating(name string, rd *RepoRevisionData) error { return nil }
func (fcn FakeChatNotifier) Destroying(name string, rd *RepoRevisionData, reason QADestroyReason) error {
	return nil
}
func (fcn FakeChatNotifier) Updating(name string, rd *RepoRevisionData, commitmsg string) error {
	return nil
}
func (fcn FakeChatNotifier) Success(name string, hostname string, svcports map[string]int64, k8sNamespace string, rd *RepoRevisionData) error {
	return nil
}
func (fcn FakeChatNotifier) Failure(name string, rd *RepoRevisionData, msg string) error { return nil }
func (fcn FakeChatNotifier) LockError(rd *RepoRevisionData, msg string) error            { return nil }

type FakeSlackUsernameMapper struct {
	F func(githubUsername string) (string, error)
}

var _ SlackUsernameMapper = &FakeSlackUsernameMapper{}

func (fsum *FakeSlackUsernameMapper) UsernameFromGithubUsername(githubUsername string) (string, error) {
	if fsum.F != nil {
		return fsum.F(githubUsername)
	}
	return "", nil
}

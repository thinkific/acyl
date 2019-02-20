package spawner

import "context"

// AcylBackend defines the different operations that can be performed
// on an Acyl backend.
type AcylBackend interface {
	CreateEnvironment(ctx context.Context, qa *QAEnvironment, qat *QAType) (string, error)
	DestroyEnvironment(ctx context.Context, qae *QAEnvironment, dns bool) error
}

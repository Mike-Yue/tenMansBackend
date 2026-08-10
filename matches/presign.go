package matches

import "context"

// Presigner produces an upload URL for a storage key. Today only a stub exists;
// a real S3 (aws-sdk-go-v2) implementation will slot in behind this interface
// once the bucket is provisioned — the endpoint shape won't change.
type Presigner interface {
	PresignUpload(ctx context.Context, storageKey string) (string, error)
}

// stubPresigner returns an empty URL. It lets the initiate endpoint work
// end-to-end (create the pending match, hand back the storage key) before S3
// exists; the client simply has no URL to PUT to yet.
type stubPresigner struct{}

// NewStubPresigner returns a Presigner that produces no real upload URL.
func NewStubPresigner() Presigner { return stubPresigner{} }

func (stubPresigner) PresignUpload(_ context.Context, _ string) (string, error) {
	// TODO: real S3 presign once the bucket + credentials are configured.
	return "", nil
}

package networking

// We single out the status here as it's actually tracked by the block fetching process in block authority.
// By doing it like that, we can handle concurrency inside block authority and don't have to use a
// mutex structure with getters/setters.
type ManagedVerifierStatus struct {
	FastFetchMode                     bool
	QueriedLastInterval               bool
	LastBlockRequestedTime            int64
	QueryHistory                      []int
	QueryIndex                        int
	ConsecutiveSuccessfulBlockFetches int
	WaitingForAnswer                  bool
}

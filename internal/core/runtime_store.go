package core

// RuntimeStore is the complete storage contract required by the ircintel Core
// process. A backend is not eligible for runtime selection until it implements
// this entire interface.
type RuntimeStore interface {
	ObservationStore
	ObservationReader
	EndpointStatusReader
	NetworkStatusReader
	NetworkIncidentReader
	NetworkIncidentStatsReader
	NetworkIncidentStatsSnapshotReader
	IncidentReader
	RegistryReader
	RegistryWriter
	DiscoveryCandidateStore
	DiscoveryReviewStore
	DiscoveryPromotionStore
	Close() error
}

var _ RuntimeStore = (*SQLiteStore)(nil)
var _ RuntimeStore = (*PostgresStore)(nil)

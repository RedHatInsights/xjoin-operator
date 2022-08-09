package kafka

type TopicParameters struct {
	Replicas           int
	Partitions         int
	CleanupPolicy      string
	MinCompactionLagMS string
	RetentionBytes     string
	RetentionMS        string
	MessageBytes       string
	CreationTimeout    int
}

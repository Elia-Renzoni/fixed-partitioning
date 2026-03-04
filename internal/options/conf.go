package options

type ProjectOptions struct {
	CoordinatorAddress string `yaml:"coordinator"`
	ReplicationFactor  int    `yaml:"replication_factor"`
	HashSlots          int    `yaml:"hash_slots"`
	ServerAddress      string `yaml:"server_address"`
}

func (p ProjectOptions) GetCoordinatorAddress() string {
	return p.CoordinatorAddress
}

func (p ProjectOptions) GetReplicationFactor() int {
	return p.ReplicationFactor
}

func (p ProjectOptions) GetHashSlots() int {
	return p.HashSlots
}

func (p ProjectOptions) GetServerAddress() string {
	return p.ServerAddress
}

package failover

import (
	"io/ioutil"

	"github.com/BurntSushi/toml"
)

const (
	ClusterStateNew      = "new"
	ClusterStateExisting = "existing"
)

const (
	MastersStateNew      = "new"
	MastersStateExisting = "existing"
)

type RaftConfig struct {
	Addr         string   `toml:"addr"`
	DataDir      string   `toml:"data_dir"`
	LogDir       string   `toml:"log_dir"`
	Cluster      []string `toml:"cluster"`
	ClusterState string   `toml:"cluster_state"`
}

type ZkConfig struct {
	Addr    []string `toml:"addr"`
	BaseDir string   `toml:"base_dir"`
}

type Config struct {
	Addr          string   `toml:"addr"`
	Masters       []string `toml:"masters"`
	MastersState  string   `toml:"masters_state"`
	CheckInterval int      `toml:"check_interval"`
	MaxDownTime   int      `toml:"max_down_time"`

	Broker string     `toml:"broker"`
	Raft   RaftConfig `toml:"raft"`
	Zk     ZkConfig   `toml:"zk"`
}

func NewConfigWithFile(name string) (*Config, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	return NewConfig(string(data))
}

func NewConfig(data string) (*Config, error) {
	var c Config

	_, err := toml.Decode(data, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

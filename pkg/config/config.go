package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/openshift/microshift/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

const (
	defaultUserConfigFile   = "~/.microshift/config.yaml"
	defaultUserDataDir      = "~/.microshift/data"
	defaultUserLogDir       = "~/.microshift/log"
	defaultGlobalConfigFile = "/etc/microshift/config.yaml"
	defaultGlobalDataDir    = "/var/lib/microshift"
	defaultGlobalLogDir     = "/var/log"
)

var (
	defaultRoles = validRoles
	validRoles   = []string{"controlplane", "node"}
)

type ClusterConfig struct {
	URL string `yaml:"url"`

	ClusterCIDR string `yaml:"clusterCIDR"`
	ServiceCIDR string `yaml:"serviceCIDR"`
	DNS         string `yaml:"dns"`
	Domain      string `yaml:"domain"`
}

type ControlPlaneConfig struct {
	// Token string `yaml:"token", envconfig:"CONTROLPLANE_TOKEN"`
}

type NodeConfig struct {
	// Token string `yaml:"token", envconfig:"NODE_TOKEN"`
}

type MicroshiftConfig struct {
	ConfigFile string
	DataDir    string `yaml:"dataDir"`
	LogDir     string `yaml:"logDir"`

	Roles []string `yaml:"roles"`

	HostName string `yaml:"nodeName"`
	HostIP   string `yaml:"nodeIP"`

	Cluster      ClusterConfig      `yaml:"cluster"`
	ControlPlane ControlPlaneConfig `yaml:"controlPlane"`
	Node         NodeConfig         `yaml:"node"`
}

func NewMicroshiftConfig() *MicroshiftConfig {
	hostName, err := os.Hostname()
	if err != nil {
		logrus.Fatalf("failed to get hostname: %v", err)
	}
	hostIP, err := util.GetHostIP()
	if err != nil {
		logrus.Fatalf("failed to get host IP: %v", err)
	}

	return &MicroshiftConfig{
		ConfigFile: findConfigFile(),
		DataDir:    findDataDir(),
		LogDir:     findLogDir(),
		Roles:      defaultRoles,
		HostName:   hostName,
		HostIP:     hostIP,
		Cluster: ClusterConfig{
			URL:         "https://127.0.0.1:6443",
			ClusterCIDR: "10.42.0.0/16",
			ServiceCIDR: "10.43.0.0/16",
			DNS:         "10.43.0.10",
			Domain:      "cluster.local",
		},
		ControlPlane: ControlPlaneConfig{},
		Node:         NodeConfig{},
	}
}

// Returns the default user config file if that exists, else the default global
// global config file, else the empty string.
func findConfigFile() string {
	userConfigFile, _ := homedir.Expand(defaultUserConfigFile)
	if _, err := os.Stat(userConfigFile); errors.Is(err, os.ErrNotExist) {
		if _, err := os.Stat(defaultGlobalConfigFile); errors.Is(err, os.ErrNotExist) {
			return ""
		} else {
			return defaultGlobalConfigFile
		}
	} else {
		return userConfigFile
	}
}

// Returns the default user data dir if it exists or the user is non-root.
// Returns the default global data dir otherwise.
func findDataDir() string {
	userDataDir, _ := homedir.Expand(defaultUserDataDir)
	if _, err := os.Stat(userDataDir); errors.Is(err, os.ErrNotExist) {
		if os.Geteuid() > 0 {
			return userDataDir
		} else {
			return defaultGlobalDataDir
		}
	} else {
		return userDataDir
	}
}

// Returns the default user log dir if the default user _data_ dir exists or the user is non-root.
// Returns the default global log dir otherwise.
func findLogDir() string {
	userDataDir, _ := homedir.Expand(defaultUserDataDir)
	userLogDir, _ := homedir.Expand(defaultUserLogDir)
	if _, err := os.Stat(userDataDir); errors.Is(err, os.ErrNotExist) {
		if os.Geteuid() > 0 {
			return userLogDir
		} else {
			return defaultGlobalLogDir
		}
	} else {
		return userLogDir
	}
}

func StringInList(s string, list []string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (c *MicroshiftConfig) ReadFromConfigFile() error {
	if len(c.ConfigFile) == 0 {
		return nil
	}

	f, err := os.Open(c.ConfigFile)
	if err != nil {
		return fmt.Errorf("opening config file %s: %v", c.ConfigFile, err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(c); err != nil {
		return fmt.Errorf("decoding config file %s: %v", c.ConfigFile, err)
	}

	return nil
}

func (c *MicroshiftConfig) ReadFromEnv() error {
	return envconfig.Process("microshift", c)
}

func (c *MicroshiftConfig) ReadFromCmdLine(flags *pflag.FlagSet) error {
	if dataDir, err := flags.GetString("data-dir"); err == nil && flags.Changed("data-dir") {
		c.DataDir = dataDir
	}
	if roles, err := flags.GetStringSlice("roles"); err == nil && flags.Changed("roles") {
		c.Roles = roles
	}
	return nil
}

func (c *MicroshiftConfig) ReadAndValidate(flags *pflag.FlagSet) error {
	if err := c.ReadFromConfigFile(); err != nil {
		return err
	}
	if err := c.ReadFromEnv(); err != nil {
		return err
	}
	if err := c.ReadFromCmdLine(flags); err != nil {
		return err
	}

	for _, role := range c.Roles {
		if !StringInList(role, validRoles) {
			return fmt.Errorf("config error: '%s' is not a valid role, must be in ['controlplane','node']", role)
		}
	}

	return nil
}

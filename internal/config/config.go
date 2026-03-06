package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Namespace         string            `yaml:"namespace"`
	DatabaseName      string            `yaml:"database_name"`
	GrafanaPassword   string            `yaml:"grafana_password"`
	SampleAppReplicas int               `yaml:"sample_app_replicas"`
	Cluster           ClusterConfig     `yaml:"cluster"`
	Storage           map[string]string `yaml:"storage"`
}

type ClusterConfig struct {
	Name  string         `yaml:"name" json:"name"`
	Ports map[string]int `yaml:"ports" json:"ports"`
}

type Manifest struct {
	Images map[string]string `yaml:"images"`
}

func Load(configPath, manifestPath string) (*Config, *Manifest, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, nil, err
	}
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return nil, nil, err
	}
	return config, manifest, nil
}

func loadConfig(path string) (*Config, error) {
	deployConfigPath := "deploy-config.yaml"
	if fileExists(deployConfigPath) {
		return loadConfigWithMerging(path, deployConfigPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadConfigWithMerging(configPath, deployConfigPath string) (*Config, error) {
	deployData, err := os.ReadFile(deployConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load deploy config: %v", err)
	}
	var deployConfig struct {
		Paths struct {
			Shared string `yaml:"shared"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(deployData, &deployConfig); err != nil {
		return nil, fmt.Errorf("failed to parse deploy config: %v", err)
	}
	sharedData, err := os.ReadFile(deployConfig.Paths.Shared)
	if err != nil {
		return nil, fmt.Errorf("failed to load shared config: %v", err)
	}
	var sharedConfig Config
	if err := yaml.Unmarshal(sharedData, &sharedConfig); err != nil {
		return nil, fmt.Errorf("failed to parse shared config: %v", err)
	}
	tfData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load terraform config: %v", err)
	}
	var tfConfig Config
	if err := yaml.Unmarshal(tfData, &tfConfig); err != nil {
		return nil, fmt.Errorf("failed to parse terraform config: %v", err)
	}
	merged := sharedConfig
	if merged.Storage == nil {
		merged.Storage = make(map[string]string)
	}
	if merged.Cluster.Ports == nil {
		merged.Cluster.Ports = make(map[string]int)
	}
	if tfConfig.Namespace != "" {
		merged.Namespace = tfConfig.Namespace
	}
	if tfConfig.DatabaseName != "" {
		merged.DatabaseName = tfConfig.DatabaseName
	}
	if tfConfig.GrafanaPassword != "" {
		merged.GrafanaPassword = tfConfig.GrafanaPassword
	}
	if tfConfig.SampleAppReplicas != 0 {
		merged.SampleAppReplicas = tfConfig.SampleAppReplicas
	}
	if tfConfig.Cluster.Name == "" {
		merged.Cluster = sharedConfig.Cluster
	} else {
		merged.Cluster = tfConfig.Cluster
	}
	if len(tfConfig.Storage) > 0 {
		merged.Storage = tfConfig.Storage
	}
	return &merged, nil
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

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

type TerraformVars struct {
	Namespace         string            `json:"namespace"`
	Images            map[string]string `json:"images"`
	Cluster           ClusterConfig     `json:"cluster"`
	Storage           map[string]string `json:"storage"`
	DatabaseName      string            `json:"database_name"`
	GrafanaPassword   string            `json:"grafana_password"`
	SampleAppReplicas int               `json:"sample_app_replicas"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: terraformlib <plan|apply|destroy> [-c <config>] [-m <manifest>] [-s <terraform-dir>]")
		os.Exit(1)
	}

	action := os.Args[1]
	configFile := ""
	manifestFile := ""
	terraformDir := "."

	// Parse flags
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-c":
			if i+1 < len(os.Args) {
				configFile = os.Args[i+1]
				i++
			}
		case "-m":
			if i+1 < len(os.Args) {
				manifestFile = os.Args[i+1]
				i++
			}
		case "-s":
			if i+1 < len(os.Args) {
				terraformDir = os.Args[i+1]
				i++
			}
		}
	}

	// Check if terraform.json exists in .terraform directory for apply/destroy
	terraformJsonPath := filepath.Join(terraformDir, ".terraform", "terraform.json")
	if (action == "apply" || action == "destroy") && fileExists(terraformJsonPath) {
		// If terraform.json exists, we can run without config/manifest
		fmt.Printf("Using existing %s for %s operation\n", terraformJsonPath, action)
		if err := runTerraformDirectly(action, terraformDir); err != nil {
			log.Fatal(err)
		}
		return
	}

	// All other operations require config and manifest
	if configFile == "" || manifestFile == "" {
		fmt.Println("Config and manifest files required for this operation")
		fmt.Println("Usage: terraformlib <plan|apply|destroy> -c <config> -m <manifest> [-s <terraform-dir>]")
		os.Exit(1)
	}

	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}

	manifest, err := loadManifest(manifestFile)
	if err != nil {
		log.Fatal(err)
	}

	tfVars := TerraformVars{
		Namespace:         config.Namespace,
		Images:            manifest.Images,
		Cluster:           config.Cluster,
		Storage:           config.Storage,
		DatabaseName:      config.DatabaseName,
		GrafanaPassword:   config.GrafanaPassword,
		SampleAppReplicas: config.SampleAppReplicas,
	}

	// Ensure basic dependencies are available
	if err := ensureBasicDependencies(); err != nil {
		log.Fatal(err)
	}

	if err := runTerraformInDir(action, tfVars, terraformDir); err != nil {
		log.Fatal(err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runTerraformDirectly(action string, terraformDir string) error {
	// Change to terraform directory
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(terraformDir); err != nil {
		return fmt.Errorf("failed to change to terraform directory %s: %v", terraformDir, err)
	}

	if action == "plan" || action == "apply" {
		if err := exec.Command("terraform", "init").Run(); err != nil {
			return err
		}
	}

	cmd := exec.Command("terraform", action)
	if action == "apply" || action == "destroy" {
		cmd.Args = append(cmd.Args, "-auto-approve")
	}

	// Use the existing JSON file from .terraform directory
	if action == "plan" || action == "apply" || action == "destroy" {
		cmd.Args = append(cmd.Args, "-var-file=.terraform/terraform.json")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ensureBasicDependencies() error {
	dependencies := []struct {
		name    string
		command string
		install func() error
	}{
		{"terraform", "terraform", installTerraform},
		{"docker", "docker", installDocker},
	}

	for _, dep := range dependencies {
		if !commandExists(dep.command) {
			fmt.Printf("Installing %s...\n", dep.name)
			if err := dep.install(); err != nil {
				return fmt.Errorf("failed to install %s: %v", dep.name, err)
			}
		}
	}
	return nil
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func installTerraform() error {
	fmt.Println("Installing terraform via package manager...")
	cmd := exec.Command("sh", "-c", "wget -O- https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg && echo 'deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com jammy main' | sudo tee /etc/apt/sources.list.d/hashicorp.list && sudo apt update && sudo apt install -y terraform")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installDocker() error {
	fmt.Println("Installing docker...")
	cmd := exec.Command("sh", "-c", "curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh get-docker.sh && sudo usermod -aG docker $USER")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	fmt.Println("Note: You may need to log out and back in for docker group membership to take effect")
	return nil
}

func loadConfig(path string) (*Config, error) {
	// Check if there's a deploy-config.yaml to handle merging
	deployConfigPath := "deploy-config.yaml"
	if fileExists(deployConfigPath) {
		return loadConfigWithMerging(path, deployConfigPath)
	}
	
	// Fallback to direct loading
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
	// Load deploy config to get paths
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
	
	// Load shared values
	sharedData, err := os.ReadFile(deployConfig.Paths.Shared)
	if err != nil {
		return nil, fmt.Errorf("failed to load shared config: %v", err)
	}
	
	var sharedConfig Config
	if err := yaml.Unmarshal(sharedData, &sharedConfig); err != nil {
		return nil, fmt.Errorf("failed to parse shared config: %v", err)
	}
	
	// Load terraform-specific config
	tfData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load terraform config: %v", err)
	}
	
	var tfConfig Config
	if err := yaml.Unmarshal(tfData, &tfConfig); err != nil {
		return nil, fmt.Errorf("failed to parse terraform config: %v", err)
	}
	
	// Merge configs (terraform-specific overrides shared)
	merged := sharedConfig
	// Initialize maps if nil
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
	// Always use shared cluster config if terraform doesn't override
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

func writeTerraformVars(vars TerraformVars, terraformDir string) error {
	// Create .terraform directory if it doesn't exist
	terraformStateDir := filepath.Join(terraformDir, ".terraform")
	if err := os.MkdirAll(terraformStateDir, 0755); err != nil {
		return err
	}

	// Write as JSON (Terraform native support) in .terraform directory
	jsonData, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return err
	}

	jsonPath := filepath.Join(terraformStateDir, "terraform.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return err
	}

	// Also write as YAML for easier reading/editing in .terraform directory
	yamlData, err := yaml.Marshal(vars)
	if err != nil {
		return err
	}

	yamlPath := filepath.Join(terraformStateDir, "terraform.yaml")
	return os.WriteFile(yamlPath, yamlData, 0644)
}

func runTerraformInDir(action string, vars TerraformVars, terraformDir string) error {
	// Change to terraform directory
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(terraformDir); err != nil {
		return fmt.Errorf("failed to change to terraform directory %s: %v", terraformDir, err)
	}

	// Reset .terraform directory for clean state
	terraformStateDir := ".terraform"
	if err := os.RemoveAll(terraformStateDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to reset .terraform directory: %v", err)
	}

	// Write terraform vars in the .terraform directory
	if err := writeTerraformVars(vars, "."); err != nil {
		return err
	}

	if action == "plan" || action == "apply" {
		if err := exec.Command("terraform", "init").Run(); err != nil {
			return err
		}
	}

	cmd := exec.Command("terraform", action)
	if action == "apply" || action == "destroy" {
		cmd.Args = append(cmd.Args, "-auto-approve")
	}

	// Use the generated JSON file from .terraform directory
	if action == "plan" || action == "apply" || action == "destroy" {
		cmd.Args = append(cmd.Args, "-var-file=.terraform/terraform.json")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
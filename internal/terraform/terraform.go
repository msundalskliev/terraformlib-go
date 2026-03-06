package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/msundalskliev/terraformlib-go/internal/config"
	"gopkg.in/yaml.v2"
)

type Vars struct {
	Namespace         string               `json:"namespace"`
	Images            map[string]string    `json:"images"`
	Cluster           config.ClusterConfig `json:"cluster"`
	Storage           map[string]string    `json:"storage"`
	DatabaseName      string               `json:"database_name"`
	GrafanaPassword   string               `json:"grafana_password"`
	SampleAppReplicas int                  `json:"sample_app_replicas"`
}

func Run(action string, cfg *config.Config, manifest *config.Manifest, terraformDir string) error {
	vars := Vars{
		Namespace:         cfg.Namespace,
		Images:            manifest.Images,
		Cluster:           cfg.Cluster,
		Storage:           cfg.Storage,
		DatabaseName:      cfg.DatabaseName,
		GrafanaPassword:   cfg.GrafanaPassword,
		SampleAppReplicas: cfg.SampleAppReplicas,
	}
	if err := ensureDependencies(); err != nil {
		return err
	}
	return runInDir(action, vars, terraformDir)
}

func RunDirect(action, terraformDir string) error {
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
	if action == "plan" || action == "apply" || action == "destroy" {
		cmd.Args = append(cmd.Args, "-var-file=.terraform/terraform.json")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func JsonExists(terraformDir string) bool {
	path := filepath.Join(terraformDir, ".terraform", "terraform.json")
	_, err := os.Stat(path)
	return err == nil
}

func ensureDependencies() error {
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
			fmt.Printf("Installing %s...\\n", dep.name)
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

func writeVars(vars Vars, terraformDir string) error {
	terraformStateDir := filepath.Join(terraformDir, ".terraform")
	if err := os.MkdirAll(terraformStateDir, 0755); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return err
	}
	jsonPath := filepath.Join(terraformStateDir, "terraform.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return err
	}
	yamlData, err := yaml.Marshal(vars)
	if err != nil {
		return err
	}
	yamlPath := filepath.Join(terraformStateDir, "terraform.yaml")
	return os.WriteFile(yamlPath, yamlData, 0644)
}

func runInDir(action string, vars Vars, terraformDir string) error {
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(terraformDir); err != nil {
		return fmt.Errorf("failed to change to terraform directory %s: %v", terraformDir, err)
	}
	terraformStateDir := ".terraform"
	if err := os.RemoveAll(terraformStateDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to reset .terraform directory: %v", err)
	}
	if err := writeVars(vars, "."); err != nil {
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
	if action == "plan" || action == "apply" || action == "destroy" {
		cmd.Args = append(cmd.Args, "-var-file=.terraform/terraform.json")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

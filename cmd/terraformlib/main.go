package main

import (
	"fmt"
	"log"
	"os"

	"github.com/msundalskliev/terraformlib-go/internal/config"
	"github.com/msundalskliev/terraformlib-go/internal/terraform"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: terraformlib <plan|apply|destroy> [-c <config>] [-m <manifest>] [-s <terraform-dir>]")
		os.Exit(1)
	}
	action := os.Args[1]
	configFile := ""
	manifestFile := ""
	terraformDir := "."
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
	if (action == "apply" || action == "destroy") && terraform.JsonExists(terraformDir) {
		fmt.Printf("Using existing terraform.json for %s operation\n", action)
		if err := terraform.RunDirect(action, terraformDir); err != nil {
			log.Fatal(err)
		}
		return
	}
	if configFile == "" || manifestFile == "" {
		fmt.Println("Config and manifest files required for this operation")
		fmt.Println("Usage: terraformlib <plan|apply|destroy> -c <config> -m <manifest> [-s <terraform-dir>]")
		os.Exit(1)
	}
	cfg, manifest, err := config.Load(configFile, manifestFile)
	if err != nil {
		log.Fatal(err)
	}
	if err := terraform.Run(action, cfg, manifest, terraformDir); err != nil {
		log.Fatal(err)
	}
}

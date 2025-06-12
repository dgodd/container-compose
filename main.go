package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Services map[string]Service `yaml:"services"`
}

type Service struct {
	Image       string   `yaml:"image"`
	Platform    string   `yaml:"platform"`
	Ports       []string `yaml:"ports"`
	Environment []string `yaml:"environment"`
	Command     []string `yaml:"command"`
	Volumes     []string `yaml:"volumes"`
	Deploy      struct {
		Resources struct {
			Limits struct {
				Memory string `yaml:"memory"`
			} `yaml:"limits"`
		} `yaml:"resources"`
	} `yaml:"deploy"`
}

func getConfig() (*Config, error) {
	var config Config
	fh, err := os.Open("docker-compose.yml")
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	yaml.NewDecoder(fh).Decode(&config)
	return &config, nil
}

type Network struct {
	Address  string `json:"address"`
	Gateway  string `json:"gateway"`
	Hostname string `json:"hostname"`
	Network  string `json:"network"`
}

type InspectData struct {
	Status   string    `json:"status"`
	Networks []Network `json:"networks"`
}

func inspectService(name string) (*InspectData, error) {
	r, w := io.Pipe()
	decoder := json.NewDecoder(r)
	cmd := exec.Command("container", "inspect", name)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	var inspectData []InspectData
	err = decoder.Decode(&inspectData)
	if err != nil {
		return nil, err
	}
	err = cmd.Wait()
	if err != nil {
		return nil, err
	}
	if len(inspectData) == 0 {
		return nil, fmt.Errorf("no data found for service %s", name)
	}
	return &inspectData[0], nil
}

func startService(name string, service *Service) error {
	log.Printf("Starting service %s\n", name)

	// TODO: better name (eg. prefix name with service)
	args := []string{"run", "--name", name, "--detach", "--rm"}
	if service.Platform != "" {
		arch := strings.TrimPrefix(service.Platform, "linux/")
		args = append(args, "--arch", arch)
	}
	if service.Deploy.Resources.Limits.Memory != "" {
		args = append(args, "--memory", service.Deploy.Resources.Limits.Memory)
	}

	// TODO: HOW??
	// Ports []string `yaml:"ports"`

	for _, env := range service.Environment {
		args = append(args, "--env", env)
	}
	for _, volume := range service.Volumes {
		args = append(args, "--volume", volume)
	}
	// TODO: Is the below correct??
	// if len(Service.Command) > 0 {
	// 	args = append(args, "--entrypoint", Service.Command[0])
	// 	args = append(args, Service.Command[1:]...)
	// }
	args = append(args, service.Image)
	// fmt.Println(strings.Join(args, " "))
	cmd := exec.Command("container", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

func main() {
	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <start|status|stop>", os.Args[0])
	}

	switch os.Args[1] {
	case "start":
		for name, service := range config.Services {
			inspectData, err := inspectService(name)
			if err == nil && inspectData.Status == "running" {
				log.Printf("Service %s is already running\n", name)
				continue
			}
			err = startService(name, &service)
			if err != nil {
				log.Fatal(err)
			}
		}
	case "status":
		for name, _ := range config.Services {
			inspectData, err := inspectService(name)
			if err != nil {
				log.Printf("Service %s: %s\n", name, err)
			} else {
				log.Printf("Service %s: %s\n", name, inspectData.Status)
			}
		}
	case "stop":
		for name, _ := range config.Services {
			log.Printf("Stopping service %s\n", name)
			err := exec.Command("container", "stop", name).Run()
			if err != nil {
				log.Fatal(err)
			}
		}
	default:
		log.Fatalf("Usage: %s <start|status|stop>", os.Args[0])
	}
}

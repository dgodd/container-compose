package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name     string              `yaml:"name"`
	Services map[string]*Service `yaml:"services"`
}

type Service struct {
	Name     string `yaml:"-"`
	Image    string `yaml:"image"`
	Platform string `yaml:"platform"`
	Ports       []string `yaml:"ports"`
	Workdir     string   `yaml:"working_dir"`
	Environment []string `yaml:"environment"`
	Command     []string `yaml:"command"`
	Entrypoint  string   `yaml:"entrypoint"`
	DependsOn   []string `yaml:"depends_on"`
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
	if config.Name == "" {
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = filepath.Base(dir)
		config.Name = dir
	}
	for name, service := range config.Services {
		service.Name = config.Name + "-" + name
	}
	return &config, nil
}

type Network struct {
	Address  string `json:"address"`
	Gateway  string `json:"gateway"`
	Hostname string `json:"hostname"`
	Network  string `json:"network"`
}

type InspectConfigImage struct {
	Reference string `json:"reference"`
}

type InspectConfiguration struct {
	Image InspectConfigImage `json:"image"`
}

type InspectData struct {
	Status        string              `json:"status"`
	Networks      []Network           `json:"networks"`
	Configuration InspectConfiguration `json:"configuration"`
}

func (service *Service) Inspect() (*InspectData, error) {
	r, w := io.Pipe()
	decoder := json.NewDecoder(r)
	cmd := exec.Command("container", "inspect", service.Name)
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
		return nil, fmt.Errorf("no data found for service %s", service.Name)
	}
	return &inspectData[0], nil
}

func (service *Service) Start(detach bool, runArgs []string) error {
	// TODO: better name (eg. prefix name with service)
	args := []string{"run", "--name", service.Name, "--rm", "--dns-domain", "test"}
	if detach {
		args = append(args, "--detach")
	}
	if service.Platform != "" {
		arch := strings.TrimPrefix(service.Platform, "linux/")
		args = append(args, "--arch", arch)
	}
	if service.Workdir != "" {
		args = append(args, "--workdir", service.Workdir)
	}
	if service.Deploy.Resources.Limits.Memory != "" {
		args = append(args, "--memory", service.Deploy.Resources.Limits.Memory)
	}
	for _, port := range service.Ports {
		args = append(args, "-p", port)
	}
	for _, env := range service.Environment {
		args = append(args, "--env", env)
	}
	for _, volume := range service.Volumes {
		if strings.HasPrefix(volume, "./") {
			volume = strings.Replace(volume, "./", "", 1)
		}
		if !strings.HasPrefix(volume, "/") {
			path, err := os.Getwd()
			if err != nil {
				return err
			}
			volume = filepath.Join(path, volume)
		}
		args = append(args, "--volume", volume)
	}
	if service.Entrypoint != "" {
		args = append(args, "--entrypoint", service.Entrypoint)
	}
	args = append(args, service.Image)
	args = append(args, service.Command...)
	args = append(args, runArgs...)
	// fmt.Println("container", strings.Join(args, " "))
	cmd := exec.Command("container", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (service *Service) Stop() error {
	return exec.Command("container", "stop", service.Name).Run()
}

func (service *Service) StartExisting() error {
	log.Printf("Starting existing container %s\n", service.Name)
	cmd := exec.Command("container", "start", service.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (service *Service) Logs(follow bool, tail int) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", tail))
	}
	args = append(args, service.Name)
	cmd := exec.Command("container", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func imageTagsDiffer(configured, reference string) bool {
	// Extract tag from configured image (default "latest")
	cfgTag := "latest"
	if idx := strings.LastIndex(configured, ":"); idx != -1 {
		cfgTag = configured[idx+1:]
	}

	// Extract tag from reference
	refTag := "latest"
	if idx := strings.LastIndex(reference, ":"); idx != -1 {
		refTag = reference[idx+1:]
	}

	return cfgTag != refTag
}

func sortedServices(config *Config) []*Service {
	// Map names to services for lookup
	byName := make(map[string]*Service)
	for _, svc := range config.Services {
		byName[svc.Name] = svc
	}

	// Topological sort using Kahn's algorithm
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // service -> services that depend on it

	for _, svc := range config.Services {
		if _, ok := inDegree[svc.Name]; !ok {
			inDegree[svc.Name] = 0
		}
		for _, dep := range svc.DependsOn {
			inDegree[svc.Name]++
			dependents[dep] = append(dependents[dep], svc.Name)
		}
	}

	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var sorted []*Service
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		sorted = append(sorted, byName[name])
		for _, dep := range dependents[name] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// Append any services not in the sort (e.g. circular deps) at the end
	for _, svc := range config.Services {
		found := false
		for _, s := range sorted {
			if s.Name == svc.Name {
				found = true
				break
			}
		}
		if !found {
			sorted = append(sorted, svc)
		}
	}

	return sorted
}

func main() {
	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	if !(len(os.Args) == 2 || (len(os.Args) >= 3 && os.Args[1] == "run")) {
		log.Fatalf("Usage: %s <start|status|run|stop|logs>", os.Args[0])
	}

	switch os.Args[1] {
	case "start":
		var alerts []string
		for _, service := range sortedServices(config) {
			name := service.Name
			inspectData, err := service.Inspect()
			if err == nil {
				// Container exists
				if inspectData.Configuration.Image.Reference != "" {
					if imageTagsDiffer(service.Image, inspectData.Configuration.Image.Reference) {
						alert := fmt.Sprintf("WARNING: Service %s is using image %s but docker-compose.yml specifies %s",
							name, inspectData.Configuration.Image.Reference, service.Image)
						log.Println(alert)
						alerts = append(alerts, alert)
					}
				}

				if inspectData.Status == "running" {
					log.Printf("Service %s is already running\n", name)
					continue
				}

				if err := service.StartExisting(); err != nil {
					log.Println("ERROR:", err)
				}
			} else {
				// Container doesn't exist, create a new one
				log.Printf("Starting service %s\n", service.Name)
				if err := service.Start(true, nil); err != nil {
					log.Println("ERROR:", err)
				}
			}
		}
		if len(alerts) > 0 {
			log.Println("--- Image version alerts ---")
			for _, alert := range alerts {
				log.Println(alert)
			}
		}
	case "status":
		for _, service := range sortedServices(config) {
			inspectData, err := service.Inspect()
			if err != nil {
				log.Printf("Service %s: %s\n", service.Name, err)
			} else {
				log.Printf("Service %s: %s\n", service.Name, inspectData.Status)
			}
		}
	case "run":
		for name, service := range config.Services {
			if name == os.Args[2] {
				log.Printf("Running service %s\n", service.Name)
				if err := service.Start(false, os.Args[3:]); err != nil {
					log.Fatal(err)
				}
				log.Printf("Service %s started\n", name)
			}
		}
	case "stop":
		// Stop in reverse dependency order (dependents first, then dependencies)
		sorted := sortedServices(config)
		for i := len(sorted) - 1; i >= 0; i-- {
			service := sorted[i]
			log.Printf("Stopping service %s\n", service.Name)
			err := service.Stop()
			if err != nil {
				log.Fatal(err)
			}
		}
	case "logs":
		follow := false
		tail := 0
		args := os.Args[2:]
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "-f", "--follow":
				follow = true
			case "-n":
				if i+1 < len(args) {
					fmt.Sscanf(args[i+1], "%d", &tail)
					i++
				}
			default:
				log.Fatalf("Usage: %s logs [-f] [-n N]", os.Args[0])
			}
		}
		for _, service := range sortedServices(config) {
			err := service.Logs(follow, tail)
			if err != nil {
				log.Println("ERROR:", err)
			}
		}
	default:
		log.Fatalf("Usage: %s <start|status|run|stop|logs>", os.Args[0])
	}
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type EnvVar struct {
	VariableType     string `json:"variable_type"`
	Key              string `json:"key"`
	Value            string `json:"value"`
	Protected        bool   `json:"protected"`
	Masked           bool   `json:"masked"`
	EnvironmentScope string `json:"environment_scope"`
}

type GitLabClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewGitLabClient(baseURL, token string) *GitLabClient {
	return &GitLabClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

func (c *GitLabClient) makeRequest(method, path string, body io.Reader) (*http.Request, error) {
	url := fmt.Sprintf("%s/api/v4/%s", c.baseURL, path)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (c *GitLabClient) GetVariables(projectPath string) ([]EnvVar, error) {
	encodedPath := url.PathEscape(projectPath)
	path := fmt.Sprintf("projects/%s/variables", encodedPath)

	req, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("project not found: %s", projectPath)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get variables: status code %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var variables []EnvVar
	if err := json.NewDecoder(resp.Body).Decode(&variables); err != nil {
		return nil, err
	}

	return variables, nil
}

func (c *GitLabClient) CreateVariable(projectPath string, variable EnvVar, dryRun bool) error {
	if dryRun {
		return nil
	}

	encodedPath := url.PathEscape(projectPath)
	data, err := json.Marshal(variable)
	if err != nil {
		return err
	}

	req, err := c.makeRequest("POST", fmt.Sprintf("projects/%s/variables", encodedPath), strings.NewReader(string(data)))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create variable %s: status code %d, response: %s", variable.Key, resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func writeDryRunOutput(filename string, sourceProject string, targetProject string, variables []EnvVar) error {
	output := struct {
		Timestamp     string   `json:"timestamp"`
		SourceProject string   `json:"source_project"`
		TargetProject string   `json:"target_project"`
		Variables     []EnvVar `json:"variables"`
	}{
		Timestamp:     time.Now().Format(time.RFC3339),
		SourceProject: sourceProject,
		TargetProject: targetProject,
		Variables:     variables,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func main() {
	var (
		gitlabURL     = flag.String("gitlab-url", "", "GitLab instance URL (e.g., https://gitlab.com)")
		token         = flag.String("token", "", "GitLab access token")
		sourceProject = flag.String("source", "", "Source project path (e.g., group/project)")
		targetProject = flag.String("target", "", "Target project path (e.g., group/project)")
		dryRun        = flag.Bool("dry-run", false, "Perform a dry run and write output to file")
		outputFile    = flag.String("output", "env-sync-dry-run.json", "Output file for dry run (default: env-sync-dry-run.json)")
	)

	flag.Parse()

	if *gitlabURL == "" || *token == "" || *sourceProject == "" || *targetProject == "" {
		flag.Usage()
		fmt.Println("\nExample usage:")
		fmt.Println("  ./gitlab-env-sync \\")
		fmt.Println("    --gitlab-url https://gitlab.com \\")
		fmt.Println("    --token your-token \\")
		fmt.Println("    --source group/project-a \\")
		fmt.Println("    --target group/project-b")
		os.Exit(1)
	}

	*gitlabURL = strings.TrimRight(*gitlabURL, "/")

	client := NewGitLabClient(*gitlabURL, *token)

	log.Printf("Fetching variables from source project: %s", *sourceProject)
	sourceVars, err := client.GetVariables(*sourceProject)
	if err != nil {
		log.Fatalf("Error getting variables from source project: %v", err)
	}

	if *dryRun {
		log.Printf("Performing dry run, writing output to %s", *outputFile)
		if err := writeDryRunOutput(*outputFile, *sourceProject, *targetProject, sourceVars); err != nil {
			log.Fatalf("Error writing dry run output: %v", err)
		}
		log.Printf("Dry run completed. Found %d variables to transfer", len(sourceVars))
		return
	}

	log.Printf("Starting transfer of %d variables from %s to %s", len(sourceVars), *sourceProject, *targetProject)

	successCount := 0
	for _, v := range sourceVars {
		log.Printf("Transferring variable: %s", v.Key)
		if err := client.CreateVariable(*targetProject, v, false); err != nil {
			log.Printf("Error transferring variable %s: %v", v.Key, err)
			continue
		}
		successCount++
	}

	log.Printf("Transfer completed. Successfully transferred %d/%d variables", successCount, len(sourceVars))
}


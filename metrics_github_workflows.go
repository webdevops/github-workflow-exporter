package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/go-github/v92/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
	"golang.org/x/exp/slices"
)

const (
	CUSTOMPROP_LABEL_FMT = "prop_%s"
	LABEL_VALUE_UNKNOWN  = "<unknown>"
)

type (
	MetricsCollectorGithubWorkflows struct {
		collector.Processor

		prometheus struct {
			repository *prometheus.GaugeVec
			workflow   *prometheus.GaugeVec

			workflowRunRunning          *prometheus.GaugeVec
			workflowRunRunningStartTime *prometheus.GaugeVec

			workflowLatestRun          *prometheus.GaugeVec
			workflowLatestRunStartTime *prometheus.GaugeVec
			workflowLatestRunDuration  *prometheus.GaugeVec

			workflowConsecutiveFailures *prometheus.GaugeVec
			workflowConsecutiveRuns     *prometheus.GaugeVec
		}
	}
)

func (m *MetricsCollectorGithubWorkflows) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	var customPropLabels []string
	for _, customProp := range Opts.GitHub.Repositories.CustomProperties {
		customPropLabels = append(customPropLabels, fmt.Sprintf(CUSTOMPROP_LABEL_FMT, customProp))
	}

	// ##############################################################3
	// Infrastructure

	m.prometheus.repository = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_repository_info",
			Help: "GitHub repository info",
		},
		append(
			[]string{
				"org",
				"repo",
				"defaultBranch",
			},
			customPropLabels...,
		),
	)
	m.Collector.RegisterMetricList("repository", m.prometheus.repository, true)

	m.prometheus.workflow = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_info",
			Help: "GitHub workflow info",
		},
		append(
			[]string{
				"org",
				"repo",
				"workflowID",
				"workflow",
				"workflowUrl",
				"workflowDefinitionUrl",
				"state",
				"path",
			},
			customPropLabels...,
		),
	)
	m.Collector.RegisterMetricList("workflow", m.prometheus.workflow, true)

	// ##############################################################3
	// Workflow run running

	m.prometheus.workflowRunRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_run_running",
			Help: "GitHub workflow running information",
		},
		[]string{
			"org",
			"repo",
			"workflowID",
			"workflowRunNumber",
			"workflow",
			"workflowUrl",
			"workflowDefinitionUrl",
			"workflowRun",
			"workflowRunUrl",
			"event",
			"branch",
			"status",
			"actorLogin",
			"actorType",
		},
	)
	m.Collector.RegisterMetricList("workflowRunRunning", m.prometheus.workflowRunRunning, true)

	m.prometheus.workflowRunRunningStartTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_run_running_start_time_seconds",
			Help: "GitHub workflow run running start time as unix timestamp",
		},
		[]string{
			"org",
			"repo",
			"workflowID",
			"workflowRunNumber",
		},
	)
	m.Collector.RegisterMetricList("workflowRunRunningStartTime", m.prometheus.workflowRunRunningStartTime, true)

	// ##############################################################3
	// Workflow run latest

	m.prometheus.workflowLatestRun = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_latest_run",
			Help: "GitHub workflow latest run information",
		},
		[]string{
			"org",
			"repo",
			"workflowID",
			"workflowRunNumber",
			"workflow",
			"workflowUrl",
			"workflowDefinitionUrl",
			"workflowRun",
			"workflowRunUrl",
			"event",
			"branch",
			"conclusion",
			"actorLogin",
			"actorType",
		},
	)
	m.Collector.RegisterMetricList("workflowLatestRun", m.prometheus.workflowLatestRun, true)

	m.prometheus.workflowLatestRunStartTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_latest_run_start_time_seconds",
			Help: "GitHub workflow latest run last executed timestamp",
		},
		[]string{
			"org",
			"repo",
			"workflowID",
			"workflowRunNumber",
		},
	)
	m.Collector.RegisterMetricList("workflowLatestRunStartTime", m.prometheus.workflowLatestRunStartTime, true)

	m.prometheus.workflowLatestRunDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_latest_run_duration_seconds",
			Help: "GitHub workflow latest run last duration in seconds",
		},
		[]string{
			"org",
			"repo",
			"workflowID",
			"workflowRunNumber",
		},
	)
	m.Collector.RegisterMetricList("workflowLatestRunDuration", m.prometheus.workflowLatestRunDuration, true)

	// ##############################################################3
	// Workflow consecutive failed runs

	m.prometheus.workflowConsecutiveFailures = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_consecutive_failed_runs",
			Help: "GitHub workflow consecutive count of failed runs per workflow",
		},
		[]string{
			"org",
			"repo",
			"workflowID",
			"workflowRunNumber",
			"workflow",
			"workflowUrl",
			"workflowDefinitionUrl",
			"workflowRun",
			"workflowRunUrl",
			"branch",
		},
	)
	m.Collector.RegisterMetricList("workflowConsecutiveFailures", m.prometheus.workflowConsecutiveFailures, true)

	// ##############################################################3
	// Workflow consecutive failed runs

	m.prometheus.workflowConsecutiveRuns = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_consecutive_runs",
			Help: "GitHub workflow consecutive count of failed runs per workflow",
		},
		[]string{
			"org",
			"repo",
			"workflowID",
			"workflowRunNumber",
			"workflow",
			"workflowUrl",
			"workflowDefinitionUrl",
			"workflowRun",
			"workflowRunUrl",
			"branch",
			"status",
		},
	)
	m.Collector.RegisterMetricList("workflowConsecutiveRuns", m.prometheus.workflowConsecutiveRuns, true)
}

func (m *MetricsCollectorGithubWorkflows) Reset() {}

func (m *MetricsCollectorGithubWorkflows) getRepoList(ctx context.Context, org string) ([]*github.Repository, error) {
	var repositories []*github.Repository

	opts := github.RepositoryListByOrgOptions{
		Page:    1,
		PerPage: 100,
	}

	for {
		m.Logger().Debug(`fetching repository list`, slog.Int("page", opts.Page))

		result, response, err := githubClient.Repositories.ListByOrg(ctx, org, &opts)
		if err != nil {
			return repositories, err
		}

		repositories = append(repositories, result...)

		// calc next page
		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	if len(Opts.GitHub.Repositories.CustomProperties) >= 1 {
		for _, repository := range repositories {
			var err error
			var repoCustomProperties []*github.CustomPropertyValue
			for {
				repoCustomProperties, _, err = githubClient.Repositories.GetAllCustomPropertyValues(ctx, org, repository.GetName())
				if err != nil {
					panic(err)
				}
				break
			}

			repository.CustomProperties = map[string]any{}
			for _, property := range repoCustomProperties {
				repository.CustomProperties[property.PropertyName] = property.GetValue()
			}
		}
	}

	return repositories, nil
}

func (m *MetricsCollectorGithubWorkflows) getRepoWorkflows(ctx context.Context, org string, repo *github.Repository) (*GithubWorkflowList, error) {
	workflows := NewGithubWorkflowList(org, repo)

	opts := github.ListOptions{PerPage: 100, Page: 1}

	for {
		m.Logger().Debug(`fetching workflows list for repository`, slog.String("repository", repo.GetName()), slog.Int("page", opts.Page))

		result, response, err := githubClient.Actions.ListWorkflows(ctx, org, repo.GetName(), &opts)
		if err != nil {
			return workflows, err
		}

		workflows.Add(result.Workflows...)

		// calc next page
		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	return workflows, nil
}

func (m *MetricsCollectorGithubWorkflows) getRepoWorkflowRuns(ctx context.Context, org string, repo *github.Repository) (*GithubWorkflowRunList, error) {
	workflowRuns := NewGithubWorkflowRunList(org, repo)

	opts := github.ListWorkflowRunsOptions{
		Branch:              repo.GetDefaultBranch(),
		ExcludePullRequests: true,
		Page:                1,
		PerPage:             100,
		Created:             ">=" + time.Now().Add(-Opts.GitHub.Workflows.Timeframe).Format(time.RFC3339),
	}

	for {
		m.Logger().Debug(`fetching list of workflow runs for repository`, slog.String("repository", repo.GetName()), slog.Int("page", opts.Page))

		result, response, err := githubClient.Actions.ListRepositoryWorkflowRuns(ctx, Opts.GitHub.Organization, *repo.Name, &opts)
		if err != nil {
			return workflowRuns, err
		}

		workflowRuns.Add(result.WorkflowRuns...)

		// calc next page
		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	return workflowRuns, nil
}

func (m *MetricsCollectorGithubWorkflows) Collect(callback chan<- func()) {
	repositoryMetric := m.Collector.GetMetricList("repository")
	workflowMetric := m.Collector.GetMetricList("workflow")

	ctx := context.WithValue(m.Context(), github.SleepUntilPrimaryRateLimitResetWhenRateLimited, true)

	org := Opts.GitHub.Organization

	repositories, err := m.getRepoList(ctx, org)
	if err != nil {
		panic(err)
	}

	for _, repo := range repositories {
		// skip archived or disabled repos
		if repo.GetArchived() || repo.GetDisabled() {
			continue
		}

		// skip repos without default branch (not setup yet?)
		if repo.GetDefaultBranch() == "" {
			// repo doesn't have default branch
			continue
		}

		// build custom properties
		propLabels := prometheus.Labels{}
		if len(Opts.GitHub.Repositories.CustomProperties) >= 1 {
			for _, customProp := range Opts.GitHub.Repositories.CustomProperties {
				labelName := fmt.Sprintf(CUSTOMPROP_LABEL_FMT, customProp)
				propLabels[labelName] = ""

				if val, exists := repo.CustomProperties[customProp]; exists {
					propLabels[labelName] = fmt.Sprintf("%v", val)
				}
			}
		}

		// repo info metric
		labels := prometheus.Labels{
			"org":           org,
			"repo":          repo.GetName(),
			"defaultBranch": to.String(repo.DefaultBranch),
		}
		for labelName, labelValue := range propLabels {
			labels[labelName] = labelValue
		}
		repositoryMetric.AddInfo(labels)

		// get workflows
		workflows, err := m.getRepoWorkflows(ctx, org, repo)
		if err != nil {
			panic(err)
		}

		// workflow info metrics
		for _, workflow := range workflows.List() {
			labels := prometheus.Labels{
				"org":        org,
				"repo":       repo.GetName(),
				"workflowID": fmt.Sprintf("%v", workflow.GetID()),
				"state":      workflow.GetState(),
				"path":       workflow.GetPath(),
			}
			workflow.AddDefaultLabels(labels)
			for labelName, labelValue := range propLabels {
				labels[labelName] = labelValue
			}
			workflowMetric.AddInfo(labels)
		}

		if workflows.Count() >= 1 {
			workflowRuns, err := m.getRepoWorkflowRuns(ctx, org, repo)
			if err != nil {
				panic(err)
			}

			if workflowRuns.Count() >= 1 {
				m.collectRunningRuns(Opts.GitHub.Organization, repo, workflows, workflowRuns, callback)
				m.collectLatestRun(Opts.GitHub.Organization, repo, workflows, workflowRuns, callback)
				m.collectConsecutiveFailures(Opts.GitHub.Organization, repo, workflows, workflowRuns, callback)
				m.collectConsecutiveRuns(Opts.GitHub.Organization, repo, workflows, workflowRuns, callback)
			}
		}
	}
}

func (m *MetricsCollectorGithubWorkflows) collectRunningRuns(org string, repo *github.Repository, workflows *GithubWorkflowList, workflowRunList *GithubWorkflowRunList, callback chan<- func()) {
	runMetric := m.Collector.GetMetricList("workflowRunRunning")
	runStartTimeMetric := m.Collector.GetMetricList("workflowRunRunningStartTime")

	for _, row := range workflowRunList.List() {
		workflowRun := row

		// ignore non-running workflows //nolint
		if !slices.Contains(Opts.GitHub.Workflows.RunRunningIgnoreStatus, strings.ToLower(workflowRun.GetStatus())) {
			continue
		}

		if workflowRun.GetConclusion() != "" {
			// skip if task has a conclusion
			continue
		}

		infoLabels := prometheus.Labels{
			"org":               org,
			"repo":              repo.GetName(),
			"workflowID":        fmt.Sprintf("%v", workflowRun.GetWorkflowID()),
			"workflowRunNumber": fmt.Sprintf("%v", workflowRun.GetRunNumber()),
			"workflow":          LABEL_VALUE_UNKNOWN,
			"workflowUrl":       "",
			"workflowRun":       workflowRun.GetName(),
			"workflowRunUrl":    workflowRun.GetHTMLURL(),
			"event":             workflowRun.GetEvent(),
			"branch":            workflowRun.GetHeadBranch(),
			"status":            workflowRun.GetStatus(),
			"actorLogin":        workflowRun.Actor.GetLogin(),
			"actorType":         workflowRun.Actor.GetType(),
		}

		// add workflow labels
		workflows.GetById(workflowRun.GetWorkflowID()).AddDefaultLabels(infoLabels)

		statLabels := prometheus.Labels{
			"org":               org,
			"repo":              repo.GetName(),
			"workflowID":        fmt.Sprintf("%v", workflowRun.GetWorkflowID()),
			"workflowRunNumber": fmt.Sprintf("%v", workflowRun.GetRunNumber()),
		}

		runMetric.AddInfo(infoLabels)
		runStartTimeMetric.AddTime(statLabels, workflowRun.GetRunStartedAt().Time)
	}
}

func (m *MetricsCollectorGithubWorkflows) collectLatestRun(org string, repo *github.Repository, workflows *GithubWorkflowList, workflowRunList *GithubWorkflowRunList, callback chan<- func()) {
	runMetric := m.Collector.GetMetricList("workflowLatestRun")
	runTimestampMetric := m.Collector.GetMetricList("workflowLatestRunStartTime")
	runDurationMetric := m.Collector.GetMetricList("workflowLatestRunDuration")

	latestJobs := map[int64]*GitHubWorkflowRun{}
	for _, row := range workflowRunList.List() {
		workflowRun := row
		workflowId := workflowRun.GetWorkflowID()

		workflow := workflows.GetById(workflowRun.GetWorkflowID())
		if !workflow.IsActive() {
			continue
		}

		// skip forks
		if workflowRun.GetHeadRepository().Fork != nil && *workflowRun.GetHeadRepository().Fork {
			continue
		}

		// ignore non-running workflows //nolint
		if !slices.Contains(Opts.GitHub.Workflows.RunLatestIgnoreStatus, strings.ToLower(workflowRun.GetStatus())) {
			continue
		}

		if workflowRun.GetConclusion() == "" {
			// skip empty conclusions or runs which are currently running
			continue
		}

		if _, exists := latestJobs[workflowId]; !exists {
			latestJobs[workflowId] = workflowRun
		} else if latestJobs[workflowId].GetCreatedAt().Before(workflowRun.GetCreatedAt().Time) {
			latestJobs[workflowId] = workflowRun
		}
	}

	for _, workflowRun := range latestJobs {
		infoLabels := prometheus.Labels{
			"org":               org,
			"repo":              repo.GetName(),
			"workflowID":        fmt.Sprintf("%v", workflowRun.GetWorkflowID()),
			"workflowRunNumber": fmt.Sprintf("%v", workflowRun.GetRunNumber()),
			"workflow":          LABEL_VALUE_UNKNOWN,
			"workflowUrl":       "",
			"workflowRun":       workflowRun.GetName(),
			"workflowRunUrl":    workflowRun.GetHTMLURL(),
			"event":             workflowRun.GetEvent(),
			"branch":            workflowRun.GetHeadBranch(),
			"conclusion":        workflowRun.GetConclusion(),
			"actorLogin":        workflowRun.Actor.GetLogin(),
			"actorType":         workflowRun.Actor.GetType(),
		}

		// add workflow labels
		workflows.GetById(workflowRun.GetWorkflowID()).AddDefaultLabels(infoLabels)

		statLabels := prometheus.Labels{
			"org":               org,
			"repo":              repo.GetName(),
			"workflowID":        fmt.Sprintf("%v", workflowRun.GetWorkflowID()),
			"workflowRunNumber": fmt.Sprintf("%v", workflowRun.GetRunNumber()),
		}

		runMetric.AddInfo(infoLabels)
		runTimestampMetric.AddTime(statLabels, workflowRun.GetRunStartedAt().Time)
		runDurationMetric.Add(statLabels, workflowRun.GetUpdatedAt().Sub(workflowRun.GetCreatedAt().Time).Seconds())
	}
}

func (m *MetricsCollectorGithubWorkflows) collectConsecutiveFailures(org string, repo *github.Repository, workflows *GithubWorkflowList, workflowRunList *GithubWorkflowRunList, callback chan<- func()) {
	consecutiveFailuresMetric := m.Collector.GetMetricList("workflowConsecutiveFailures")

	consecutiveFailMap := map[int64]*struct {
		count  int64
		labels prometheus.Labels
	}{}
	consecutiveFinishedMap := map[int64]bool{}

	for _, row := range workflowRunList.List() {
		workflowRun := row
		workflowId := workflowRun.GetWorkflowID()

		workflow := workflows.GetById(workflowRun.GetWorkflowID())
		if !workflow.IsActive() {
			continue
		}

		// ignore non-running workflows //nolint
		if !slices.Contains(Opts.GitHub.Workflows.ConsecutiveFailuresIgnoreStatus, strings.ToLower(workflowRun.GetStatus())) {
			continue
		}

		if _, exists := consecutiveFailMap[workflowId]; !exists {
			infoLabels := prometheus.Labels{
				"org":               org,
				"repo":              repo.GetName(),
				"workflowID":        fmt.Sprintf("%v", workflowRun.GetWorkflowID()),
				"workflowRunNumber": fmt.Sprintf("%v", workflowRun.GetRunNumber()),
				"workflow":          LABEL_VALUE_UNKNOWN,
				"workflowUrl":       "",
				"workflowRun":       workflowRun.GetName(),
				"workflowRunUrl":    workflowRun.GetHTMLURL(),
				"branch":            workflowRun.GetHeadBranch(),
			}

			// add workflow labels
			workflow.AddDefaultLabels(infoLabels)

			consecutiveFailMap[workflowId] = &struct {
				count  int64
				labels prometheus.Labels
			}{
				count:  0,
				labels: infoLabels,
			}
			consecutiveFinishedMap[workflowId] = false
		}

		// successful run found for workload id, skipping all further runs
		if consecutiveFinishedMap[workflowId] {
			continue
		}

		switch workflowRun.GetConclusion() {
		case "":
			continue
		case "failure":
			consecutiveFailMap[workflowId].count++
		case "success":
			consecutiveFinishedMap[workflowId] = true
		}
	}

	// process metrics
	for _, row := range consecutiveFailMap {
		consecutiveFailuresMetric.Add(row.labels, float64(row.count))
	}
}

func (m *MetricsCollectorGithubWorkflows) collectConsecutiveRuns(org string, repo *github.Repository, workflows *GithubWorkflowList, workflowRunList *GithubWorkflowRunList, callback chan<- func()) {
	consecutiveRunsMetric := m.Collector.GetMetricList("workflowConsecutiveRuns")

	consecutiveRunMap := map[int64]*struct {
		count  int64
		status string
		labels prometheus.Labels
	}{}
	consecutiveFinishedMap := map[int64]bool{}

	for _, row := range workflowRunList.List() {
		workflowRun := row
		workflowId := workflowRun.GetWorkflowID()

		workflow := workflows.GetById(workflowRun.GetWorkflowID())
		if !workflow.IsActive() {
			continue
		}

		// ignore non-running workflows //nolint
		if !slices.Contains(Opts.GitHub.Workflows.ConsecutiveRunIgnoreStatus, strings.ToLower(workflowRun.GetStatus())) {
			continue
		}

		if _, exists := consecutiveRunMap[workflowId]; !exists {
			infoLabels := prometheus.Labels{
				"org":               org,
				"repo":              repo.GetName(),
				"workflowID":        fmt.Sprintf("%v", workflowRun.GetWorkflowID()),
				"workflowRunNumber": fmt.Sprintf("%v", workflowRun.GetRunNumber()),
				"workflow":          LABEL_VALUE_UNKNOWN,
				"workflowUrl":       "",
				"workflowRun":       workflowRun.GetName(),
				"workflowRunUrl":    workflowRun.GetHTMLURL(),
				"branch":            workflowRun.GetHeadBranch(),
				"status":            workflowRun.GetStatus(),
			}

			// add workflow labels
			workflow.AddDefaultLabels(infoLabels)

			consecutiveRunMap[workflowId] = &struct {
				count  int64
				status string
				labels prometheus.Labels
			}{
				count:  0,
				status: workflowRun.GetStatus(),
				labels: infoLabels,
			}
			consecutiveFinishedMap[workflowId] = false
		}

		// successful run found for workload id, skipping all further runs
		if consecutiveFinishedMap[workflowId] {
			continue
		}

		if strings.EqualFold(consecutiveRunMap[workflowId].status, workflowRun.GetStatus()) {
			consecutiveRunMap[workflowId].count++
		} else {
			consecutiveFinishedMap[workflowId] = true
		}
	}

	// process metrics
	for _, row := range consecutiveRunMap {
		consecutiveRunsMetric.Add(row.labels, float64(row.count))
	}
}

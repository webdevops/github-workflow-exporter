package main

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v61/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
	"golang.org/x/exp/slices"
)

const (
	CUSTOMPROP_LABEL_FMT = "prop_%s"
	LABEL_VALUE_UNKNOWN  = "<unknown>"
)

var (
	githubWorkflowRunningStatus = []string{"in_progress", "action_required", "queued", "waiting", "pending"}
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
			"workflowRun",
			"workflowRunUrl",
			"branch",
			"actorLogin",
			"actorType",
		},
	)
	m.Collector.RegisterMetricList("workflowConsecutiveFailures", m.prometheus.workflowConsecutiveFailures, true)
}

func (m *MetricsCollectorGithubWorkflows) Reset() {}

func (m *MetricsCollectorGithubWorkflows) getRepoList(org string) ([]*github.Repository, error) {
	var repositories []*github.Repository

	opts := github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100, Page: 1},
	}

	for {
		m.Logger().Debug(`fetching repository list`, slog.Int("page", opts.Page))

		result, response, err := githubClient.Repositories.ListByOrg(m.Context(), org, &opts)
		var ghRateLimitError *github.RateLimitError
		if ok := errors.As(err, &ghRateLimitError); ok {
			m.Logger().Debug("request ListByOrg rate limited", slog.Time("waitingUntil", ghRateLimitError.Rate.Reset.Time))
			time.Sleep(time.Until(ghRateLimitError.Rate.Reset.Time))
			continue
		} else if err != nil {
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
				repoCustomProperties, _, err = githubClient.Repositories.GetAllCustomPropertyValues(m.Context(), org, repository.GetName())
				var ghRateLimitError *github.RateLimitError
				if ok := errors.As(err, &ghRateLimitError); ok {
					m.Logger().Debug("request GetAllCustomPropertyValues rate limited", slog.Time("waitingUntil", ghRateLimitError.Rate.Reset.Time))
					time.Sleep(time.Until(ghRateLimitError.Rate.Reset.Time))
					continue
				} else if err != nil {
					panic(err)
				}
				break
			}

			repository.CustomProperties = map[string]string{}
			for _, property := range repoCustomProperties {
				repository.CustomProperties[property.PropertyName] = property.GetValue()
			}
		}
	}

	return repositories, nil
}

func (m *MetricsCollectorGithubWorkflows) getRepoWorkflows(org, repo string) (map[int64]*github.Workflow, error) {
	workflows := map[int64]*github.Workflow{}

	opts := github.ListOptions{PerPage: 100, Page: 1}

	for {
		m.Logger().Debug(`fetching workflows list for repository`, slog.String("repository", repo), slog.Int("page", opts.Page))

		result, response, err := githubClient.Actions.ListWorkflows(m.Context(), org, repo, &opts)
		var ghRateLimitError *github.RateLimitError
		if ok := errors.As(err, &ghRateLimitError); ok {
			m.Logger().Debug("request ListWorkflows rate limited", slog.Time("waitingUntil", ghRateLimitError.Rate.Reset.Time))
			time.Sleep(time.Until(ghRateLimitError.Rate.Reset.Time))
			continue
		} else if err != nil {
			return workflows, err
		}

		for _, row := range result.Workflows {
			workflows[row.GetID()] = row
		}

		// calc next page
		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	return workflows, nil
}

func (m *MetricsCollectorGithubWorkflows) getRepoWorkflowRuns(repo *github.Repository) ([]*github.WorkflowRun, error) {
	var workflowRuns []*github.WorkflowRun

	opts := github.ListWorkflowRunsOptions{
		Branch:              repo.GetDefaultBranch(),
		ExcludePullRequests: true,
		ListOptions:         github.ListOptions{PerPage: 100, Page: 1},
		Created:             ">=" + time.Now().Add(-Opts.GitHub.Workflows.Timeframe).Format(time.RFC3339),
	}

	for {
		m.Logger().Debug(`fetching list of workflow runs for repository`, slog.String("repository", repo.GetName()), slog.Int("page", opts.Page))

		result, response, err := githubClient.Actions.ListRepositoryWorkflowRuns(m.Context(), Opts.GitHub.Organization, *repo.Name, &opts)
		var ghRateLimitError *github.RateLimitError
		if ok := errors.As(err, &ghRateLimitError); ok {
			m.Logger().Debug("request ListRepositoryWorkflowRuns rate limited", slog.Time("waitingUntil", ghRateLimitError.Rate.Reset.Time))
			time.Sleep(time.Until(ghRateLimitError.Rate.Reset.Time))
			continue
		} else if err != nil {
			return workflowRuns, err
		}

		workflowRuns = append(workflowRuns, result.WorkflowRuns...)

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

	org := Opts.GitHub.Organization

	repositories, err := m.getRepoList(org)
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
					propLabels[labelName] = val
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
		workflows, err := m.getRepoWorkflows(org, repo.GetName())
		if err != nil {
			panic(err)
		}

		// workflow info metrics
		for _, workflow := range workflows {
			labels := prometheus.Labels{
				"org":         org,
				"repo":        repo.GetName(),
				"workflowID":  fmt.Sprintf("%v", workflow.GetID()),
				"workflow":    workflow.GetName(),
				"state":       workflow.GetState(),
				"path":        workflow.GetPath(),
				"workflowUrl": workflow.GetHTMLURL(),
			}
			for labelName, labelValue := range propLabels {
				labels[labelName] = labelValue
			}
			workflowMetric.AddInfo(labels)
		}

		if len(workflows) >= 1 {
			workflowRuns, err := m.getRepoWorkflowRuns(repo)
			if err != nil {
				panic(err)
			}

			if len(workflowRuns) >= 1 {
				m.collectRunningRuns(Opts.GitHub.Organization, repo, workflows, workflowRuns, callback)
				m.collectLatestRun(Opts.GitHub.Organization, repo, workflows, workflowRuns, callback)
				m.collectConsecutiveFailures(Opts.GitHub.Organization, repo, workflows, workflowRuns, callback)
			}
		}
	}
}

func (m *MetricsCollectorGithubWorkflows) collectRunningRuns(org string, repo *github.Repository, workflows map[int64]*github.Workflow, workflowRun []*github.WorkflowRun, callback chan<- func()) {
	runMetric := m.Collector.GetMetricList("workflowRunRunning")
	runStartTimeMetric := m.Collector.GetMetricList("workflowRunRunningStartTime")

	for _, row := range workflowRun {
		workflowRun := row

		// ignore non running workflows
		if !slices.Contains(githubWorkflowRunningStatus, workflowRun.GetStatus()) {
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

		if workflow, ok := workflows[workflowRun.GetWorkflowID()]; ok {
			infoLabels["workflow"] = workflow.GetName()
			infoLabels["workflowUrl"] = workflow.GetHTMLURL()
		}

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

func (m *MetricsCollectorGithubWorkflows) collectLatestRun(org string, repo *github.Repository, workflows map[int64]*github.Workflow, workflowRun []*github.WorkflowRun, callback chan<- func()) {
	runMetric := m.Collector.GetMetricList("workflowLatestRun")
	runTimestampMetric := m.Collector.GetMetricList("workflowLatestRunStartTime")
	runDurationMetric := m.Collector.GetMetricList("workflowLatestRunDuration")

	latestJobs := map[int64]*github.WorkflowRun{}
	for _, row := range workflowRun {
		workflowRun := row
		workflowId := workflowRun.GetWorkflowID()

		// ignore running/not finished workflow runs
		if slices.Contains(githubWorkflowRunningStatus, workflowRun.GetStatus()) {
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
		if workflow, ok := workflows[workflowRun.GetWorkflowID()]; ok {
			infoLabels["workflow"] = workflow.GetName()
			infoLabels["workflowUrl"] = workflow.GetHTMLURL()
		}

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

func (m *MetricsCollectorGithubWorkflows) collectConsecutiveFailures(org string, repo *github.Repository, workflows map[int64]*github.Workflow, workflowRun []*github.WorkflowRun, callback chan<- func()) {
	consecutiveFailuresMetric := m.Collector.GetMetricList("workflowConsecutiveFailures")

	consecutiveFailMap := map[int64]*struct {
		count  int64
		labels prometheus.Labels
	}{}
	consecutiveFinishedMap := map[int64]bool{}

	for _, row := range workflowRun {
		workflowRun := row
		workflowId := workflowRun.GetWorkflowID()

		// ignore running/not finished workflow runs
		switch workflowRun.GetStatus() {
		case "in_progress":
			continue
		case "action_required":
			continue
		case "queued":
			continue
		case "waiting":
			continue
		case "pending":
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
				"actorLogin":        workflowRun.Actor.GetLogin(),
				"actorType":         workflowRun.Actor.GetType(),
			}

			if workflow, ok := workflows[workflowRun.GetWorkflowID()]; ok {
				infoLabels["workflow"] = workflow.GetName()
				infoLabels["workflowUrl"] = workflow.GetHTMLURL()
			}

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

package main

import (
	"errors"
	"time"

	"github.com/google/go-github/v61/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type (
	MetricsCollectorGithubWorkflows struct {
		collector.Processor

		prometheus struct {
			repository                  *prometheus.GaugeVec
			workflow                    *prometheus.GaugeVec
			workflowLatestRun           *prometheus.GaugeVec
			workflowLatestRunTimestamp  *prometheus.GaugeVec
			workflowConsecutiveFailures *prometheus.GaugeVec
		}
	}
)

func (m *MetricsCollectorGithubWorkflows) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.repository = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_repository_info",
			Help: "GitHub repository info",
		},
		[]string{
			"org",
			"repo",
			"defaultBranch",
		},
	)
	m.Collector.RegisterMetricList("repository", m.prometheus.repository, true)

	m.prometheus.workflow = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_info",
			Help: "GitHub workflow info",
		},
		[]string{
			"org",
			"repo",
			"workflow",
			"state",
			"path",
		},
	)
	m.Collector.RegisterMetricList("workflow", m.prometheus.workflow, true)

	m.prometheus.workflowLatestRun = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_latest_run",
			Help: "GitHub workflow latest run information",
		},
		[]string{
			"org",
			"repo",
			"workflow",
			"event",
			"branch",
			"conclusion",
		},
	)
	m.Collector.RegisterMetricList("workflowLatestRun", m.prometheus.workflowLatestRun, true)

	m.prometheus.workflowLatestRunTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_latest_run_timestamp_seconds",
			Help: "GitHub workflow latest run last executed timestamp",
		},
		[]string{
			"org",
			"repo",
			"workflow",
			"event",
			"branch",
			"conclusion",
		},
	)
	m.Collector.RegisterMetricList("workflowLatestRunTimestamp", m.prometheus.workflowLatestRunTimestamp, true)

	m.prometheus.workflowConsecutiveFailures = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_workflow_consecutive_failed_runs",
			Help: "GitHub workflow consecutive count of failed runs per workflow",
		},
		[]string{
			"org",
			"repo",
			"workflow",
			"branch",
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
		m.Logger().Debugf(`fetching repository list with page "%d"`, opts.Page)

		result, response, err := githubClient.Repositories.ListByOrg(m.Context(), org, &opts)
		var ghRateLimitError *github.RateLimitError
		if ok := errors.As(err, &ghRateLimitError); ok {
			m.Logger().Debugf("ListByOrg ratelimited. Pausing until %s", ghRateLimitError.Rate.Reset.Time.String())
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

	return repositories, nil
}

func (m *MetricsCollectorGithubWorkflows) getRepoWorkflows(org, repo string) ([]*github.Workflow, error) {
	var workflows []*github.Workflow

	opts := github.ListOptions{PerPage: 100, Page: 1}

	for {
		m.Logger().Debugf(`fetching workflows list for repo "%s" with page "%d"`, repo, opts.Page)

		result, response, err := githubClient.Actions.ListWorkflows(m.Context(), org, repo, &opts)
		var ghRateLimitError *github.RateLimitError
		if ok := errors.As(err, &ghRateLimitError); ok {
			m.Logger().Debugf("ListWorkflows ratelimited. Pausing until %s", ghRateLimitError.Rate.Reset.Time.String())
			time.Sleep(time.Until(ghRateLimitError.Rate.Reset.Time))
			continue
		} else if err != nil {
			return workflows, err
		}

		workflows = append(workflows, result.Workflows...)

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
		m.Logger().Debugf(`fetching list of workflow runs for repo "%s" with page "%d"`, repo.GetName(), opts.Page)

		result, response, err := githubClient.Actions.ListRepositoryWorkflowRuns(m.Context(), Opts.GitHub.Organization, *repo.Name, &opts)
		var ghRateLimitError *github.RateLimitError
		if ok := errors.As(err, &ghRateLimitError); ok {
			m.Logger().Debugf("ListRepositoryWorkflowRuns ratelimited. Pausing until %s", ghRateLimitError.Rate.Reset.Time.String())
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
		repositoryMetric.AddInfo(prometheus.Labels{
			"org":           org,
			"repo":          repo.GetName(),
			"defaultBranch": to.String(repo.DefaultBranch),
		})

		if repo.GetDefaultBranch() == "" {
			// repo doesn't have default branch
			continue
		}

		workflows, err := m.getRepoWorkflows(org, repo.GetName())
		if err != nil {
			panic(err)
		}

		for _, workflow := range workflows {
			workflowMetric.AddInfo(prometheus.Labels{
				"org":      org,
				"repo":     repo.GetName(),
				"workflow": workflow.GetName(),
				"state":    workflow.GetState(),
				"path":     workflow.GetPath(),
			})
		}

		if len(workflows) >= 1 {
			workflowRuns, err := m.getRepoWorkflowRuns(repo)
			if err != nil {
				panic(err)
			}

			if len(workflowRuns) >= 1 {
				m.collectLatestRun(Opts.GitHub.Organization, repo, workflowRuns, callback)
				m.collectConsecutiveFailures(Opts.GitHub.Organization, repo, workflowRuns, callback)
			}
		}
	}
}

func (m *MetricsCollectorGithubWorkflows) collectLatestRun(org string, repo *github.Repository, workflowRun []*github.WorkflowRun, callback chan<- func()) {
	runMetric := m.Collector.GetMetricList("workflowLatestRun")
	runTimestampMetric := m.Collector.GetMetricList("workflowLatestRunTimestamp")

	latestJobs := map[int64]*github.WorkflowRun{}
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
		labels := prometheus.Labels{
			"org":        org,
			"repo":       repo.GetName(),
			"workflow":   workflowRun.GetName(),
			"event":      workflowRun.GetEvent(),
			"branch":     workflowRun.GetHeadBranch(),
			"conclusion": workflowRun.GetConclusion(),
		}

		runMetric.AddInfo(labels)
		runTimestampMetric.AddTime(labels, workflowRun.GetRunStartedAt().Time)
	}
}

func (m *MetricsCollectorGithubWorkflows) collectConsecutiveFailures(org string, repo *github.Repository, workflowRun []*github.WorkflowRun, callback chan<- func()) {
	consecutiveFailuresMetric := m.Collector.GetMetricList("workflowConsecutiveFailures")

	consecutiveFailMap := map[int64]int64{}
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
			consecutiveFailMap[workflowId] = 0
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
			consecutiveFailMap[workflowId]++
		case "success":
			consecutiveFinishedMap[workflowId] = true
		}

		labels := prometheus.Labels{
			"org":      org,
			"repo":     repo.GetName(),
			"workflow": workflowRun.GetName(),
			"branch":   workflowRun.GetHeadBranch(),
		}
		consecutiveFailuresMetric.Add(labels, float64(consecutiveFailMap[workflowId]))
	}
}

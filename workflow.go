package main

import (
	"fmt"
	"strings"

	"github.com/google/go-github/v92/github"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	GithubWorkflowList struct {
		org  string
		repo *github.Repository
		list map[int64]*GitHubWorkflow
	}

	GitHubWorkflow struct {
		*github.Workflow
		ref *GithubWorkflowList
	}
)

func NewGithubWorkflowList(org string, repo *github.Repository) *GithubWorkflowList {
	return &GithubWorkflowList{
		org:  org,
		repo: repo,
		list: map[int64]*GitHubWorkflow{},
	}
}

func (list *GithubWorkflowList) Add(workflow ...*github.Workflow) {
	for _, item := range workflow {
		list.list[*item.ID] = new(GitHubWorkflow{Workflow: item, ref: list})
	}
}

func (list *GithubWorkflowList) GetById(id int64) *GitHubWorkflow {
	if workflow, ok := list.list[id]; ok {
		return workflow
	}
	return nil
}

func (list *GithubWorkflowList) Count() int {
	return len(list.list)
}

func (list *GithubWorkflowList) List() map[int64]*GitHubWorkflow {
	return list.list
}

func (workflow *GitHubWorkflow) IsActive() bool {
	if workflow != nil {
		return strings.EqualFold(workflow.GetState(), "active")
	}
	return false
}

func (workflow *GitHubWorkflow) AddDefaultLabels(labels prometheus.Labels) {
	// reset
	labels["workflow"] = ""
	labels["workflowUrl"] = ""
	labels["workflowDefinitionUrl"] = ""

	if workflow != nil {
		// for whatever reason we only get the definition url, not the real "workflow" overview page

		orgName := workflow.ref.repo.Organization.GetName()
		repoName := workflow.ref.repo.GetName()
		defaultBranch := workflow.ref.repo.GetDefaultBranch()

		workflowOverviewUrl := strings.Replace(
			workflow.GetHTMLURL(),
			fmt.Sprintf("%s/%s/blob/%s/.github/workflows/", orgName, repoName, defaultBranch),
			fmt.Sprintf("%s/%s/actions/workflows/", orgName, repoName),
			1,
		)

		labels["workflow"] = workflow.GetName()
		labels["workflowUrl"] = workflowOverviewUrl
		labels["workflowDefinitionUrl"] = workflow.GetHTMLURL()
	}
}

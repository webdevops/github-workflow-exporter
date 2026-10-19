package main

import (
	"github.com/google/go-github/v92/github"
)

type (
	GithubWorkflowRunList struct {
		org  string
		repo *github.Repository
		list []*GitHubWorkflowRun
	}

	GitHubWorkflowRun struct {
		*github.WorkflowRun
		ref *GithubWorkflowRunList
	}
)

func NewGithubWorkflowRunList(org string, repo *github.Repository) *GithubWorkflowRunList {
	return &GithubWorkflowRunList{
		org:  org,
		repo: repo,
		list: []*GitHubWorkflowRun{},
	}
}

func (list *GithubWorkflowRunList) Add(workflowRun ...*github.WorkflowRun) {
	for _, item := range workflowRun {
		list.list = append(list.list, &GitHubWorkflowRun{item, list})
	}
}
func (list *GithubWorkflowRunList) Count() int {
	return len(list.list)
}

func (list *GithubWorkflowRunList) List() []*GitHubWorkflowRun {
	return list.list
}

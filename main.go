package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"

	_ "github.com/KimMachineGun/automemlimit"
	ghinstallation "github.com/bradleyfalzon/ghinstallation/v2"
	github "github.com/google/go-github/v61/github"
	flags "github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/webdevops/go-common/prometheus/collector"

	"github.com/webdevops/github-workflow-exporter/config"
)

const (
	Author    = "webdevops.io"
	UserAgent = "github-workflow-exporter/"
)

var (
	argparser *flags.Parser
	Opts      config.Opts

	githubClient *github.Client

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
	buildDate = "<unknown>"

	// cache config
	cacheTag = "v2"
)

type Portrange struct {
	FirstPort int
	LastPort  int
}

func main() {
	initArgparser()
	initLogger()

	logger.Infof("starting github-workflows-exporter v%s (%s; %s; by %v at %v)", gitTag, gitCommit, runtime.Version(), Author, buildDate)
	logger.Info(string(Opts.GetJson()))

	initSystem()

	logger.Infof("init GitHub connection")
	initGitHubConnection()

	logger.Infof("starting metrics collection")
	initMetricCollector()

	logger.Infof("starting http server on %s", Opts.Server.Bind)
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&Opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		var flagsErr *flags.Error
		if ok := errors.As(err, &flagsErr); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}
}

func initGitHubConnection() {
	var err error

	httpClient := &http.Client{}

	if Opts.GitHub.Auth.Token != "" {
		// token auth
		logger.Info(`using GitHub token auth`)
		githubClient = github.NewClient(httpClient)
		githubClient = githubClient.WithAuthToken(Opts.GitHub.Auth.Token)
	} else if Opts.GitHub.Auth.AppID != nil {
		// app auth with private key
		logger.Info(`using GitHub app auth with private key`, slog.Int64("appID", *Opts.GitHub.Auth.AppID), slog.Int64("installationID", *Opts.GitHub.Auth.AppInstallationID))

		if Opts.GitHub.Auth.AppPrivateKeyFile == nil {
			logger.Fatal(`GitHub app private key file not specified`)
		}

		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, *Opts.GitHub.Auth.AppID, *Opts.GitHub.Auth.AppInstallationID, *Opts.GitHub.Auth.AppPrivateKeyFile)
		if err != nil {
			log.Fatal(`failed to init GitHub app auth`, slog.Any("error", err))
		}

		// adapt enterprise url
		if Opts.GitHub.EnterpriseURL != "" {
			itr.BaseURL = Opts.GitHub.EnterpriseURL

			if !strings.HasSuffix(itr.BaseURL, "/") {
				itr.BaseURL += "/"
			}
			if !strings.HasSuffix(itr.BaseURL, "/api/v3/") {
				itr.BaseURL += "api/v3/"
			}
		}

		httpClient.Transport = itr
		githubClient = github.NewClient(httpClient)
	} else {
		// no auth, failing
		logger.Fatal(`no GitHub auth specified, either use token or app based auth`)
	}

	githubClient.UserAgent = fmt.Sprintf(`%s/%s`, UserAgent, gitTag)

	if Opts.GitHub.EnterpriseURL != "" {
		githubClient, err = githubClient.WithEnterpriseURLs(Opts.GitHub.EnterpriseURL, "")
		if err != nil {
			log.Fatal(err)
		}
	}

	ctx := context.Background()

	// test connection
	logger.Info(`testing GitHub connection`)
	_, _, err = githubClient.Organizations.Get(ctx, Opts.GitHub.Organization)
	if err != nil {
		log.Fatalf(`unable to fetch GitHub org "%v": %v`, Opts.GitHub.Organization, err)
	}
}

func initMetricCollector() {
	collectorName := "workflows"
	c := collector.New(collectorName, &MetricsCollectorGithubWorkflows{}, logger.Slog())
	c.SetScapeTime(Opts.Scrape.Time)
	err := c.SetCache(
		Opts.GetCachePath(collectorName+".json"),
		collector.BuildCacheTag(cacheTag, Opts.GitHub),
	)
	if err != nil {
		logger.Fatal(err.Error())
	}
	if err := c.Start(); err != nil {
		logger.Fatal(err.Error())
	}
}

// start and handle prometheus handler
func startHttpServer() {
	mux := http.NewServeMux()

	// healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err.Error())
		}
	})

	// readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err.Error())
		}
	})

	mux.Handle("/metrics", collector.HttpWaitForRlock(promhttp.Handler()))

	srv := &http.Server{
		Addr:         Opts.Server.Bind,
		Handler:      mux,
		ReadTimeout:  Opts.Server.ReadTimeout,
		WriteTimeout: Opts.Server.WriteTimeout,
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal(err.Error())
	}
}

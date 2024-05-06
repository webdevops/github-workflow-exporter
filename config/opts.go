package config

import (
	"encoding/json"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug       bool `long:"log.debug"    env:"LOG_DEBUG"  description:"debug mode"`
			Development bool `long:"log.devel"    env:"LOG_DEVEL"  description:"development mode"`
			Json        bool `long:"log.json"     env:"LOG_JSON"   description:"Switch log output to json format"`
		}

		// Github
		GitHub struct {
			EnterpriseURL string `long:"github.enterprise.url"   env:"GITHUB_ENTERPRISE_URL"  description:"GitHub enterprise url (self hosted)"`

			Organization string `long:"github.organization"     env:"GITHUB_ORGANIZATION"    description:"GitHub organization name"`

			Auth struct {
				// PAT auth
				Token string `long:"github.token"            env:"GITHUB_TOKEN"           description:"GitHub token auth: PAT" json:"-"`

				// APP auth
				AppID             *int64  `long:"github.app.id"              env:"GITHUB_APP_ID"               description:"GitHub app auth: App ID"`
				AppInstallationID *int64  `long:"github.app.installationid"  env:"GITHUB_APP_INSTALLATION_ID"  description:"GitHub app auth: App installation ID"`
				AppPrivateKeyFile *string `long:"github.app.keyfile"         env:"GITHUB_APP_PRIVATE_KEY"      description:"GitHub app auth: Private key (path to file)"`
			}

			Workflows struct {
				Timeframe time.Duration `long:"github.workflows.timeframe"     env:"GITHUB_WORKFLOWS_TIMEFRAME"    description:"GitHub workflow timeframe for fetching" default:"168h"`
			}
		}

		Scrape struct {
			Time time.Duration `long:"scrape.time"     env:"SCRAPE_TIME"    description:"Scrape time" default:"30m"`
		}

		// caching
		Cache struct {
			Path string `long:"cache.path" env:"CACHE_PATH" description:"Cache path (to folder, file://path... or azblob://storageaccount.blob.core.windows.net/containername or k8scm://{namespace}/{configmap}})"`
		}

		Server struct {
			// general options
			Bind         string        `long:"server.bind"              env:"SERVER_BIND"           description:"Server address"        default:":8080"`
			ReadTimeout  time.Duration `long:"server.timeout.read"      env:"SERVER_TIMEOUT_READ"   description:"Server read timeout"   default:"5s"`
			WriteTimeout time.Duration `long:"server.timeout.write"     env:"SERVER_TIMEOUT_WRITE"  description:"Server write timeout"  default:"10s"`
		}
	}
)

func (o *Opts) GetCachePath(path string) (ret *string) {
	if o.Cache.Path != "" {
		tmp := o.Cache.Path + "/" + path
		ret = &tmp
	}

	return
}

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}

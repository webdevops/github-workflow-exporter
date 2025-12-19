package config

import (
	"encoding/json"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Level  string `long:"log.level"    env:"LOG_LEVEL"   description:"Log level" choice:"trace" choice:"debug" choice:"info" choice:"warning" choice:"error" default:"info"`                          // nolint:staticcheck // multiple choices are ok
			Format string `long:"log.format"   env:"LOG_FORMAT"  description:"Log format" choice:"logfmt" choice:"json" default:"logfmt"`                                                                     // nolint:staticcheck // multiple choices are ok
			Source string `long:"log.source"   env:"LOG_SOURCE"  description:"Show source for every log message (useful for debugging and bug reports)" choice:"" choice:"short" choice:"file" choice:"full"` // nolint:staticcheck // multiple choices are ok
			Color  string `long:"log.color"    env:"LOG_COLOR"   description:"Enable color for logs" choice:"" choice:"auto" choice:"yes" choice:"no"`                                                        // nolint:staticcheck // multiple choices are ok
			Time   bool   `long:"log.time"     env:"LOG_TIME"    description:"Show log time"`
		}

		// Github
		GitHub struct {
			EnterpriseURL string `long:"github.enterprise.url"   env:"GITHUB_ENTERPRISE_URL"  description:"GitHub enterprise url (self hosted)"`

			Organization string `long:"github.organization"     env:"GITHUB_ORGANIZATION"    description:"GitHub organization name" required:"true"`

			Auth struct {
				// PAT auth
				Token string `long:"github.token"            env:"GITHUB_TOKEN"           description:"GitHub token auth: PAT" json:"-"`

				// APP auth
				AppID             *int64  `long:"github.app.id"              env:"GITHUB_APP_ID"               description:"GitHub app auth: App ID"`
				AppInstallationID *int64  `long:"github.app.installationid"  env:"GITHUB_APP_INSTALLATION_ID"  description:"GitHub app auth: App installation ID"`
				AppPrivateKeyFile *string `long:"github.app.keyfile"         env:"GITHUB_APP_PRIVATE_KEY"      description:"GitHub app auth: Private key (path to file)"`
			}

			Repositories struct {
				CustomProperties []string `long:"github.repository.customprops"         env:"GITHUB_REPOSITORY_CUSTOMPROPS"      description:"GitHub repository custom properties as labels for repos and workflows (space delimiter)" env-delim:" "`
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

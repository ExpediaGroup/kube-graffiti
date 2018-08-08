package webhook

import (
	"net/url"
	"strings"

	"stash.hcom/run/kube-graffiti/pkg/log"
)

const (
	componentName = "webhook"
	pathPrefix    = "graffiti"
)

func Path(name string) *string {
	mylog := log.ComponentLogger(componentName, "Path")
	path := strings.Join([]string{pathPrefix, url.PathEscape(name)}, "/")
	mylog.Debug().Str("path", path).Msg("Generated webhook path")
	return &path
}

package blacklist

import (
	"fmt"
	"sort"
	"strings"

	"stash.hcom/run/istio-namespace-webhook/pkg/log"
)

const (
	componentName = "blacklist"
)

type Blacklist struct {
	list map[string]bool
}

// SharedBlacklist can be shared by different packages without having to pass as a variable.
var SharedBlacklist Blacklist

func New() Blacklist {
	return Blacklist{
		list: make(map[string]bool),
	}
}

func (b Blacklist) InList(value string) bool {
	mylog := log.ComponentLogger(componentName, "InList")
	r := b.list[value] == true
	mylog.Debug().Str("value", value).Bool("result", r).Str("blacklist", b.ValuesAsString()).Msg("in blacklist?")

	return b.list[value] == true
}

func (b *Blacklist) Set(values ...string) {
	mylog := log.ComponentLogger(componentName, "Set")
	for _, value := range values {
		b.list[value] = true
		mylog.Debug().Str("value", value).Msg("added to blacklist")
	}
}

func (b Blacklist) Values() []string {
	keys := make([]string, len(b.list))
	i := 0
	for k := range b.list {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func (b Blacklist) len() int {
	return len(b.list)
}

func (b Blacklist) ValuesAsString() string {
	return fmt.Sprintf("%s", strings.Join(b.Values(), ","))
}

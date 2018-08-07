package config

import (
	"stash.hcom/run/kube-graffiti/pkg/graffiti"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

type GraffitiConfig struct {
	Deployment webhook.Server
	Artists    []graffiti.Artist
}

type Artist struct {
	Registration webhook.Registration
	Tag          graffiti.Tag
}

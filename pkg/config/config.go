package config

import (
	"stash.hcom/run/kube-graffiti/pkg/graffiti"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

type GraffitiConfig struct {
	Deployment webhook.Server
	Artists    []Artist
}

type Artist struct {
	Registration webhook.Registration
	Rule         graffiti.Rule
}

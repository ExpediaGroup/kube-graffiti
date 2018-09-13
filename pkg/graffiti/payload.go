package graffiti

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

// Payload contains the actions that we would like to perform when rule matches an object, such as
// label/annotation additions or deletions, a patch or a block.
type Payload struct {
	Additions Additions `mapstructure:"additions" yaml:"additions,omitempty"`
	Deletions Deletions `mapstructure:"deletions" yaml:"deletions,omitempty"`
	Block     bool      `mapstructure:"block" yaml:"block,omitempty"`
	JSONPatch string    `mapstructure:"json-patch" yaml:"json-patch,omitempty"`
}

// Additions contains the additional fields that we want to insert into the object
// This type is directly marshalled from config and so has mapstructure tags
type Additions struct {
	Annotations map[string]string `mapstructure:"annotations" yaml:"annotations,omitempty"`
	Labels      map[string]string `mapstructure:"labels" yaml:"labels,omitempty"`
}

// Deletions contains the names of labels or annotations which you wish to remove
type Deletions struct {
	Annotations []string `mapstructure:"annotations" yaml:"annotations,omitempty"`
	Labels      []string `mapstructure:"labels" yaml:"labels,omitempty"`
}

func (p Payload) paintObject(object metaObject, fm map[string]string, logger zerolog.Logger) (patch []byte, err error) {
	mylog := logger.With().Str("func", "paintObject").Logger()

	// a block takes precedence over JSONPatch, Additions, Deletions...
	if p.Block {
		mylog.Debug().Msg("payload contains a block")
		return []byte("BLOCK"), nil
	}

	// if the user provided a patch then just use that...
	if p.JSONPatch != "" {
		mylog.Debug().Str("patch", p.JSONPatch).Msg("payload contains user provided patch")
		return []byte(p.JSONPatch), nil
	}

	// create a patch for additions + deletions
	var patchString string
	if p.containsAdditions() || p.containsDeletions() {
		mylog.Debug().Str("patch", p.JSONPatch).Msg("payload contains additions or deletions")
		patchString, err = p.processMetadataAdditionsDeletions(object, fm)
		if err != nil {
			return nil, fmt.Errorf("could not create json patch: %v", err)
		}
	}

	if patchString == "" {
		mylog.Info().Msg("paint resulted in no patch")
		return nil, nil
	} else {
		mylog.Info().Str("patch", patchString).Msg("created json patch")
	}

	return []byte(patchString), nil
}

func (p Payload) containsAdditions() bool {
	if len(p.Additions.Labels) == 0 && len(p.Additions.Annotations) == 0 {
		return false
	}
	return true
}

func (p Payload) containsDeletions() bool {
	if len(p.Deletions.Labels) == 0 && len(p.Deletions.Annotations) == 0 {
		return false
	}
	return true
}

// processMetadataAdditionsDeletions will generate a JSON patch for replacing an objects labels and/or annotations
// It is designed to replace the whole path in order to work around a bug in kubernetes that does not correctly
// unescape ~1 (/) in paths preventing annotation labels with slashes in them.
func (p Payload) processMetadataAdditionsDeletions(obj metaObject, fm map[string]string) (string, error) {
	mylog := log.ComponentLogger(componentName, "processMetadataAdditionsDeletions")
	var patches []string

	op, err := createPatchOperand(obj.Meta.Labels, p.Additions.Labels, fm, p.Deletions.Labels, "/metadata/labels")
	if err != nil {
		return "", err
	}
	if op != "" {
		mylog.Debug().Str("operand", op).Msg("created patch operand")
		patches = append(patches, op)
	}

	op, err = createPatchOperand(obj.Meta.Annotations, p.Additions.Annotations, fm, p.Deletions.Annotations, "/metadata/annotations")
	if err != nil {
		return "", err
	}
	if op != "" {
		mylog.Debug().Str("operand", op).Msg("created patch operand")
		patches = append(patches, op)
	}

	if len(patches) == 0 {
		return "", nil
	}
	return `[ ` + strings.Join(patches, ", ") + ` ]`, nil
}

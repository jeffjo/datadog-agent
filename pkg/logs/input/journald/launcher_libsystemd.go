package journald

// +build libsystemd

import (
	"strings"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-agent/pkg/logs/auditor"
	"github.com/DataDog/datadog-agent/pkg/logs/config"
	"github.com/DataDog/datadog-agent/pkg/logs/pipeline"
	"github.com/DataDog/datadog-agent/pkg/logs/restart"
)

// Launcher is in charge of starting and stopping new journald tailers
type Launcher struct {
	sources          []*config.LogSource
	pipelineProvider pipeline.Provider
	auditor          *auditor.Auditor
	tailers          map[string]*Tailer
}

// New returns a new Launcher.
func New(sources []*config.LogSource, pipelineProvider pipeline.Provider, auditor *auditor.Auditor) *Launcher {
	journaldSources := []*config.LogSource{}
	for _, source := range sources {
		if source.Config.Type == config.JournaldType {
			journaldSources = append(journaldSources, source)
		}
	}
	return &Launcher{
		sources:          journaldSources,
		pipelineProvider: pipelineProvider,
		auditor:          auditor,
		tailers:          make(map[string]*Tailer),
	}
}

// Start starts new tailers.
func (l *Launcher) Start() {
	for _, source := range l.sources {
		identifier := source.Config.Path
		if _, exists := l.tailers[identifier]; exists {
			// set up only one tailer per journal
			continue
		}
		tailer, err := l.setupTailer(source)
		if err != nil {
			log.Warn("Could not set up journald tailer: ", err)
		} else {
			l.tailers[identifier] = tailer
		}
	}
}

// Stop stops all active tailers
func (l *Launcher) Stop() {
	stopper := restart.NewParallelStopper()
	for _, tailer := range l.tailers {
		stopper.Add(tailer)
		delete(l.tailers, tailer.Identifier())
	}
	stopper.Stop()
}

// setupTailer configures and starts a new tailer,
// returns the tailer or an error.
func (l *Launcher) setupTailer(source *config.LogSource) (*Tailer, error) {
	var units []string
	if source.Config.Unit != "" {
		units = strings.Split(source.Config.Unit, ",")
	}
	config := JournalConfig{
		Units: units,
		Path:  source.Config.Path,
	}
	tailer := NewTailer(config, source, l.pipelineProvider.NextPipelineChan())
	err := tailer.Start(l.auditor.GetLastCommittedCursor(tailer.Identifier()))
	if err != nil {
		return nil, err
	}
	return tailer, nil
}
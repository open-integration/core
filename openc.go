package core

import (
	"fmt"
	"os"
	"path"

	"github.com/open-integration/core/pkg/downloader"
	"github.com/open-integration/core/pkg/logger"
	"github.com/open-integration/core/pkg/modem"
	"github.com/open-integration/core/pkg/runner"
	"github.com/open-integration/core/pkg/state"
	"github.com/open-integration/core/pkg/utils"
)

type (
	// EngineOptions to create new engine
	EngineOptions struct {
		Pipeline Pipeline
		// LogsDirectory path where to store logs
		LogsDirectory string
		Kubeconfig    *EngineKubernetesOptions
		Logger        logger.Logger
	}

	// EngineKubernetesOptions when running service on kubernetes cluster
	EngineKubernetesOptions struct {
		Path      string
		Context   string
		Namespace string
		InCluster bool
	}
)

// NewEngine create new engine
func NewEngine(opt *EngineOptions) Engine {
	e := &engine{
		pipeline: opt.Pipeline,
	}
	if opt.LogsDirectory == "" {
		wd, err := os.Getwd()
		dieOnError(err)
		opt.LogsDirectory = wd
	}

	tasksLogDir := path.Join(opt.LogsDirectory, "logs", "tasks")
	dieOnError(createDir(tasksLogDir))
	e.taskLogsDirctory = tasksLogDir

	home, err := os.UserHomeDir()
	dieOnError(err)

	servicesDir := path.Join(home, ".open-integration", "services")
	dieOnError(createDir(servicesDir))

	e.eventChan = make(chan *state.Event, 1)

	var log logger.Logger
	if opt.Logger == nil {
		loggerOptions := &logger.Options{
			FilePath:    path.Join(opt.LogsDirectory, "logs", "log"),
			LogToStdOut: true,
		}
		opt.Logger = logger.New(loggerOptions)
	}
	log = opt.Logger
	e.logger = opt.Logger.New("module", "engine")

	serviceDownloader := downloader.New(downloader.Options{
		Store:  servicesDir,
		Logger: log.New("module", "service-downloader"),
	})

	servicesLogDir := path.Join(opt.LogsDirectory, "logs", "services")
	dieOnError(createDir(servicesLogDir))

	// Init modem
	{
		e.modem = modem.New(&modem.ModemOptions{
			Logger: log.New("module", "modem"),
		})
		for _, s := range opt.Pipeline.Spec.Services {
			svcID := string(generateID())
			if opt.Kubeconfig == nil {
				location := s.Path
				if s.Name != "" && s.Version != "" {
					err := serviceDownloader.Download(s.Name, s.Version)
					dieOnError(err)
					location = path.Join(serviceDownloader.Store(), s.Name)
				}
				log.Debug("Adding service", "path", location)
				e.modem.AddService(svcID, s.As, runner.New(&runner.Options{
					Type:                 runner.LocalRunner,
					Logger:               log.New("service-runner", s.Name),
					Name:                 s.Name,
					ID:                   svcID,
					Dailer:               &utils.GRPC{},
					PortGenerator:        utils.Port{},
					LocalLogFileCreator:  &utils.FileCreator{},
					LocalLogsDirectory:   servicesLogDir,
					ServiceClientCreator: utils.Proto{},
					LocalCommandCreator:  utils.Command{},
					LocalPathToBinary:    location,
				}))
			} else {
				log.Debug("Adding service")
				runnerOpt := &runner.Options{
					Type:                     runner.KubernetesRunner,
					Logger:                   log.New("service-runner", s.Name),
					Name:                     s.Name,
					ID:                       svcID,
					Version:                  s.Version,
					PortGenerator:            utils.Port{},
					KubernetesKubeConfigPath: opt.Kubeconfig.Path,
					KubernetesContext:        opt.Kubeconfig.Context,
					KubernetesNamespace:      opt.Kubeconfig.Namespace,
					Kube:                     &utils.Kubernetes{},
					Dailer:                   &utils.GRPC{},
					ServiceClientCreator:     utils.Proto{},
				}
				if opt.Kubeconfig.InCluster {
					runnerOpt.KubernetesGrpcDialViaPodIP = true
				}
				e.modem.AddService(svcID, s.As, runner.New(runnerOpt))
			}
		}
	}
	s := state.New(&state.Options{
		Logger:       opt.Logger.New("module", "state-store"),
		EventChan:    e.eventChan,
		CommandsChan: make(chan string, 1),
		Name:         e.pipeline.Metadata.Name,
	})
	e.statev1 = s
	return e
}

func dieOnError(err error) {
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}

func createDir(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

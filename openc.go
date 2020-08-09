package core

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/open-integration/core/pkg/downloader"
	"github.com/open-integration/core/pkg/event"
	"github.com/open-integration/core/pkg/graph"
	"github.com/open-integration/core/pkg/logger"
	"github.com/open-integration/core/pkg/modem"
	"github.com/open-integration/core/pkg/service"
	"github.com/open-integration/core/pkg/state"
	"github.com/open-integration/core/pkg/utils"
)

var exit = func(code int) { os.Exit(code) }

type (
	// EngineOptions to create new engine
	EngineOptions struct {
		Pipeline Pipeline
		// LogsDirectory path where to store logs
		LogsDirectory string
		Kubeconfig    *EngineKubernetesOptions
		Logger        logger.Logger

		serviceDownloader downloader.Downloader
		modem             modem.Modem
	}

	// EngineKubernetesOptions when running service on kubernetes cluster
	EngineKubernetesOptions struct {
		Path                string
		Context             string
		Namespace           string
		InCluster           bool
		Host                string
		B64Crt              string
		Token               string
		LogsVolumeClaimName string
		LogsVolumeName      string
	}
)

// NewEngine create new engine
func NewEngine(opt *EngineOptions) Engine {

	if opt.LogsDirectory == "" {
		wd, err := os.Getwd()
		dieOnError(err)
		opt.LogsDirectory = wd
	}

	tasksLogDir := path.Join(opt.LogsDirectory, "logs", "tasks")
	dieOnError(createDir(tasksLogDir))

	home, err := os.UserHomeDir()
	dieOnError(err)

	servicesDir := path.Join(home, ".open-integration", "services")
	dieOnError(createDir(servicesDir))

	eventChannel := make(chan *event.Event, 10)

	stateUpdateChannel := make(chan state.UpdateRequest, 1)

	waitGroup := &sync.WaitGroup{}

	var log logger.Logger
	// create logger
	{
		if opt.Logger == nil {
			loggerOptions := &logger.Options{
				FilePath:    path.Join(opt.LogsDirectory, "logs", "log.log"),
				LogToStdOut: true,
			}
			log = logger.New(loggerOptions)
		} else {
			log = opt.Logger
		}
	}

	if opt.serviceDownloader == nil {
		opt.serviceDownloader = downloader.New(downloader.Options{
			Store:  servicesDir,
			Logger: log.New("module", "service-downloader"),
		})
	}

	servicesLogDir := path.Join(opt.LogsDirectory, "logs", "services")
	dieOnError(createDir(servicesLogDir))

	s := state.New(&state.Options{
		Logger:             log.New("module", "state-store"),
		EventChan:          eventChannel,
		CommandsChan:       make(chan string, 1),
		Name:               opt.Pipeline.Metadata.Name,
		StateUpdateRequest: stateUpdateChannel,
		WG:                 waitGroup,
	})
	go s.StartProcess()
	return &engine{
		statev1:            s,
		pipeline:           opt.Pipeline,
		wg:                 waitGroup,
		graphBuilder:       graph.New(),
		stateDir:           opt.LogsDirectory,
		taskLogsDirctory:   tasksLogDir,
		eventChan:          eventChannel,
		stateUpdateRequest: stateUpdateChannel,
		logger:             log.New("module", "engine"),
		modem:              createModem(opt, log, servicesLogDir),
	}
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

// HandleEngineError prints the error in case the engine.Run was failed and exit
func HandleEngineError(err error) {
	if err != nil {
		fmt.Printf("Failed to execute pipeline\nError: %v", err)
		exit(1)
	}
}

func createModem(opt *EngineOptions, log logger.Logger, servicesLogDir string) modem.Modem {
	if opt.modem != nil {
		return opt.modem
	}
	serviceModem := modem.New(&modem.Options{
		Logger: log.New("module", "modem"),
	})
	for _, s := range opt.Pipeline.Spec.Services {
		svcID := utils.GenerateID()
		if opt.Kubeconfig == nil {
			finalLocation := s.Path
			if s.Name != "" && s.Version != "" {
				location, err := opt.serviceDownloader.Download(s.Name, s.Version)
				dieOnError(err)
				finalLocation = location
			}
			log.Debug("Adding service", "path", finalLocation)
			serviceModem.AddService(s.As, service.New(&service.Options{
				Type:                 service.Local,
				Logger:               log.New("service-service", s.Name),
				Name:                 s.Name,
				ID:                   svcID,
				Dailer:               &utils.GRPC{},
				PortGenerator:        utils.Port{},
				LocalLogFileCreator:  &utils.FileCreator{},
				LogsDirectory:        servicesLogDir,
				ServiceClientCreator: utils.Proto{},
				LocalCommandCreator:  &utils.Command{},
				LocalPathToBinary:    finalLocation,
			}))
		} else {
			log.Debug("Adding service")
			runnerOpt := &service.Options{
				Type:                      service.Kubernetes,
				Logger:                    log.New("service-service", s.Name),
				Name:                      s.Name,
				ID:                        svcID,
				Version:                   s.Version,
				PortGenerator:             utils.Port{},
				KubernetesKubeConfigPath:  opt.Kubeconfig.Path,
				KubernetesContext:         opt.Kubeconfig.Context,
				KubernetesNamespace:       opt.Kubeconfig.Namespace,
				KubeconfigHost:            opt.Kubeconfig.Host,
				KubeconfigToken:           opt.Kubeconfig.Token,
				KubeconfigB64Crt:          opt.Kubeconfig.B64Crt,
				Kube:                      &utils.Kubernetes{},
				Dailer:                    &utils.GRPC{},
				ServiceClientCreator:      utils.Proto{},
				LogsDirectory:             opt.LogsDirectory,
				KubernetesVolumeClaimName: opt.Kubeconfig.LogsVolumeClaimName,
				KubernetesVolumeName:      opt.Kubeconfig.LogsVolumeName,
			}
			if opt.Kubeconfig.InCluster {
				runnerOpt.KubernetesGrpcDialViaPodIP = true
			}
			serviceModem.AddService(s.As, service.New(runnerOpt))
		}
	}
	return serviceModem
}

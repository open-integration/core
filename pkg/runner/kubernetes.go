package runner

import (
	"context"
	"fmt"

	v1 "github.com/open-integration/core/pkg/api/v1"
	"github.com/open-integration/core/pkg/logger"
	"github.com/open-integration/core/pkg/utils"
	"google.golang.org/grpc"
	apiv1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"
)

const (
	defaultPort = "8080"
)

type (
	kubernetesRunner struct {
		Logger               logger.Logger
		name                 string
		id                   string
		version              string
		kubeconfigPath       string
		kubeconfigContext    string
		kubeconfigNamespace  string
		kubeconfigHost       string
		kubeconfigB64Crt     string
		kubeconfigToken      string
		kube                 kube
		kubeclient           *kubernetes.Clientset
		dialer               dialer
		connection           *grpc.ClientConn
		serviceClientCreator serviceClientCreator
		client               v1.ServiceClient
		tasksSchemas         map[string]string
		portGenerator        portGenerator
		port                 string
		hostname             string
		grpcDialViaPodIP     bool
	}

	kube interface {
		BuildClient(utils.BuildKubeClientOptions) (*kubernetes.Clientset, error)
		BuildPodDefinition(namespace string, name string, version string, id string, port string) (*apiv1.Pod, error)
		BuildServiceDefinition(namespace string, name string, id string, port string, serviceType string) (*apiv1.Service, error)
		CreatePod(client *kubernetes.Clientset, def *apiv1.Pod) (*apiv1.Pod, error)
		WaitForPod(client *kubernetes.Clientset, pod *apiv1.Pod, phase string) error
		CreateService(client *kubernetes.Clientset, def *apiv1.Service) (*apiv1.Service, error)
		KillService(client *kubernetes.Clientset, namespace string, name string) error
		KillPod(client *kubernetes.Clientset, namespace string, name string) error
	}
)

func (_k *kubernetesRunner) Run() error {
	client, err := _k.kube.BuildClient(utils.BuildKubeClientOptions{
		KubeconfigPath: _k.kubeconfigPath,
	})
	if err != nil {
		return err
	}
	_k.kubeclient = client

	_k.Logger.Debug("Starting Kuberentes runner service-less")
	if err := _k.startService(); err != nil {
		return err
	}

	if err := _k.startPod(); err != nil {
		return err
	}

	if err := _k.dail(); err != nil {
		return err
	}

	if err := _k.init(); err != nil {
		return err
	}

	return nil
}

func (_k *kubernetesRunner) Kill() error {
	name := fmt.Sprintf("%s-%s", _k.name, _k.id)
	if err := _k.kube.KillService(_k.kubeclient, _k.kubeconfigNamespace, name); err != nil {
		_k.Logger.Warn("Failed to delete kubernetes service", "service", name, "error", err.Error())
	}

	if err := _k.kube.KillPod(_k.kubeclient, _k.kubeconfigNamespace, name); err != nil {
		_k.Logger.Warn("Failed to delete kubernetes pod", "pod", name, "error", err.Error())
	}

	return nil
}

func (_k *kubernetesRunner) Call(context context.Context, req *v1.CallRequest) (*v1.CallResponse, error) {
	return _k.client.Call(context, req)
}

func (_k *kubernetesRunner) startService() error {
	port, err := _k.portGenerator.GetAvailable()
	if err != nil {
		return err
	}
	_k.port = port
	serviceType := ""
	if !_k.grpcDialViaPodIP {
		serviceType = "LoadBalancer"
	}
	svcDef, err := _k.kube.BuildServiceDefinition(_k.kubeconfigNamespace, _k.name, _k.id, port, serviceType)
	if err != nil {
		return err
	}

	createdSvc, err := _k.kube.CreateService(_k.kubeclient, svcDef)

	if err != nil {
		return err
	}
	_k.Logger.Debug("Kubernetes Service created", "name", createdSvc.ObjectMeta.Name)
	return nil
}

func (_k *kubernetesRunner) startPod() error {
	podDef, err := _k.kube.BuildPodDefinition(_k.kubeconfigNamespace, _k.name, _k.version, _k.id, defaultPort)
	if err != nil {
		return err
	}

	createdPod, err := _k.kube.CreatePod(_k.kubeclient, podDef)
	if err != nil {
		return err
	}
	_k.Logger.Debug("Pod created", "name", createdPod.ObjectMeta.Name)

	if err := _k.kube.WaitForPod(_k.kubeclient, createdPod, "Running"); err != nil {
		return err
	}
	_k.Logger.Debug("Pod is ready", "name", createdPod.ObjectMeta.Name)

	// the target port is the default that the pod was started with
	if _k.grpcDialViaPodIP {
		_k.Logger.Debug("Updating dial options", "name", createdPod.ObjectMeta.Name, "port", _k.port, "hostname", _k.name)
		_k.hostname = fmt.Sprintf("%s-%s", _k.name, _k.id)
	}
	return nil
}

func (_k *kubernetesRunner) dail() error {
	url := fmt.Sprintf("%s:%s", _k.hostname, _k.port)
	_k.Logger.Debug("Dial to service", "URL", url)
	conn, err := _k.dialer.Dial(url, grpc.WithInsecure())
	if err != nil {
		return err
	}
	_k.connection = conn
	_k.client = _k.serviceClientCreator.New(conn)
	_k.Logger.Debug("Connection established")
	return nil
}

func (_k *kubernetesRunner) init() error {
	_k.Logger.Debug("Calling service init endpoint one time")
	resp, err := _k.client.Init(context.Background(), &v1.InitRequest{})
	if err != nil {
		return err
	}
	_k.tasksSchemas = resp.JsonSchemas
	return nil
}

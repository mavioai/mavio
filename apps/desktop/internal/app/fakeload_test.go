package app

import (
	"context"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

// GetSystemLoad serves two samples of the server's computer: its CPU a
// quarter busy, most of its memory taken, its disks without throughput
// and its network receiving.
func (s *fakeSystem) GetSystemLoad(context.Context, *systemv1.GetSystemLoadRequest) (*systemv1.GetSystemLoadResponse, error) {
	sample := func(cpu float64) *systemv1.LoadSample {
		return systemv1.LoadSample_builder{
			Cpu: systemv1.CpuLoad_builder{User: new(cpu * 0.6), System: new(cpu * 0.4), Server: new(0.05)}.Build(),
			Memory: systemv1.MemoryLoad_builder{
				TotalBytes: new(int64(16e9)), UsedBytes: new(int64(15e9)), ServerBytes: new(int64(512e6)),
			}.Build(),
			Disk:    systemv1.DiskLoad_builder{TotalBytes: new(int64(2e12)), AvailableBytes: new(int64(1.2e12))}.Build(),
			Network: systemv1.NetworkLoad_builder{ReceiveRate: new(2.5e6), SendRate: new(120e3), ReceivedBytes: new(int64(8e9)), SentBytes: new(int64(3e9))}.Build(),
		}.Build()
	}
	return systemv1.GetSystemLoadResponse_builder{Samples: []*systemv1.LoadSample{sample(0.1), sample(0.25)}}.Build(), nil
}

// ListContainers serves the TMDB plugin's container, ready, which failed
// once.
func (fakePlugins) ListContainers(context.Context, *systemv1.ListContainersRequest) (*systemv1.ListContainersResponse, error) {
	return systemv1.ListContainersResponse_builder{Containers: []*systemv1.PluginContainer{systemv1.PluginContainer_builder{
		PluginId:   new("dev.mavio.tmdb"),
		Dependency: pluginv1.ContainerDependency_builder{Name: new("cache")}.Build(),
		Status: pluginv1.ContainerStatus_builder{
			Mode: new(pluginv1.ContainerMode_CONTAINER_MODE_MANAGED), State: new(pluginv1.ContainerState_CONTAINER_STATE_READY),
			Error: new("port in use"),
		}.Build(),
		Command: new("docker run -d --name mavio-cache redis:7"),
	}.Build()}}.Build(), nil
}

func (fakePlugins) GetContainerRuntime(context.Context, *systemv1.GetContainerRuntimeRequest) (*systemv1.GetContainerRuntimeResponse, error) {
	return systemv1.GetContainerRuntimeResponse_builder{Runtime: systemv1.ContainerRuntime_builder{
		Available: new(true), Engine: new("docker"), Version: new("27.3.1"), ApiVersion: new("1.47"), Endpoint: new("unix:///var/run/docker.sock"),
	}.Build()}.Build(), nil
}

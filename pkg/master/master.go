/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package master

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/endpoint"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/minion"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/pod"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/service"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scheduler"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	goetcd "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

// Config is a structure used to configure a Master.
type Config struct {
	Client             *client.Client
	Cloud              cloudprovider.Interface
	EtcdServers        []string
	HealthCheckMinions bool
	Minions            []string
	MinionCacheTTL     time.Duration
	MinionRegexp       string
	PodInfoGetter      client.PodInfoGetter
}

// Master contains state for a Kubernetes cluster master/api server.
type Master struct {
	podRegistry        pod.Registry
	controllerRegistry controller.Registry
	serviceRegistry    service.Registry
	minionRegistry     minion.Registry
	storage            map[string]apiserver.RESTStorage
	client             *client.Client
}

// New returns a new instance of Master connected to the given etcdServer.
func New(c *Config) *Master {
	etcdClient := goetcd.NewClient(c.EtcdServers)
	minionRegistry := minionRegistryMaker(c)
	m := &Master{
		podRegistry:        etcd.NewRegistry(etcdClient, minionRegistry),
		controllerRegistry: etcd.NewRegistry(etcdClient, minionRegistry),
		serviceRegistry:    etcd.NewRegistry(etcdClient, minionRegistry),
		minionRegistry:     minionRegistry,
		client:             c.Client,
	}
	m.init(c.Cloud, c.PodInfoGetter)
	return m
}

func minionRegistryMaker(c *Config) minion.Registry {
	var minionRegistry minion.Registry
	if c.Cloud != nil && len(c.MinionRegexp) > 0 {
		var err error
		minionRegistry, err = minion.NewCloudRegistry(c.Cloud, c.MinionRegexp)
		if err != nil {
			glog.Errorf("Failed to initalize cloud minion registry reverting to static registry (%#v)", err)
		}
	}
	if minionRegistry == nil {
		minionRegistry = minion.NewRegistry(c.Minions)
	}
	if c.HealthCheckMinions {
		minionRegistry = minion.NewHealthyRegistry(minionRegistry, &http.Client{})
	}
	if c.MinionCacheTTL > 0 {
		cachingMinionRegistry, err := minion.NewCachingRegistry(minionRegistry, c.MinionCacheTTL)
		if err != nil {
			glog.Errorf("Failed to initialize caching layer, ignoring cache.")
		} else {
			minionRegistry = cachingMinionRegistry
		}
	}
	return minionRegistry
}

func (m *Master) init(cloud cloudprovider.Interface, podInfoGetter client.PodInfoGetter) {
	podCache := NewPodCache(podInfoGetter, m.podRegistry, time.Second*30)
	go podCache.Loop()

	endpoints := endpoint.NewEndpointController(m.serviceRegistry, m.client)
	go util.Forever(func() { endpoints.SyncServiceEndpoints() }, time.Second*10)

	random := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	s := scheduler.NewRandomFitScheduler(m.podRegistry, random)
	m.storage = map[string]apiserver.RESTStorage{
		"pods": pod.NewRegistryStorage(&pod.RegistryStorageConfig{
			CloudProvider: cloud,
			MinionLister:  m.minionRegistry,
			PodCache:      podCache,
			PodInfoGetter: podInfoGetter,
			Registry:      m.podRegistry,
			Scheduler:     s,
		}),
		"replicationControllers": controller.NewRegistryStorage(m.controllerRegistry, m.podRegistry),
		"services":               service.NewRegistryStorage(m.serviceRegistry, cloud, m.minionRegistry),
		"minions":                minion.NewRegistryStorage(m.minionRegistry),
	}
}

// API_v1beta1 returns the resources and codec for API version v1beta1
func (m *Master) API_v1beta1() (map[string]apiserver.RESTStorage, apiserver.Codec) {
	storage := make(map[string]apiserver.RESTStorage)
	for k, v := range m.storage {
		storage[k] = v
	}
	return storage, api.Codec
}

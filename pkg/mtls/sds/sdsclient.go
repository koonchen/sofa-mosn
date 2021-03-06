/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sds

import (
	"errors"
	"sync"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"sofastack.io/sofa-mosn/pkg/log"
	"sofastack.io/sofa-mosn/pkg/types"
)

type SdsClientImpl struct {
	SdsConfigMap   map[string]*auth.SdsSecretConfig
	SdsCallbackMap map[string]types.SdsUpdateCallbackFunc
	updatedLock    sync.Mutex
	sdsSubscriber  *SdsSubscriber
}

var sdsClient *SdsClientImpl
var sdsClientLock sync.Mutex
var sdsPostCallback func() = nil

var ErrSdsClientNotInit = errors.New("sds client not init")

// NewSdsClientSingleton use by tls module , when get sds config from xds
func NewSdsClientSingleton(config *auth.SdsSecretConfig) types.SdsClient {
	sdsClientLock.Lock()
	defer sdsClientLock.Unlock()
	if sdsClient != nil {
		return sdsClient
	} else {
		sdsClient = &SdsClientImpl{
			SdsConfigMap:   make(map[string]*auth.SdsSecretConfig),
			SdsCallbackMap: make(map[string]types.SdsUpdateCallbackFunc),
		}
		// For Istio , sds config should be the same
		// So we use first sds config to init sds subscriber
		sdsClient.sdsSubscriber = NewSdsSubscriber(sdsClient, config.SdsConfig, types.GetGlobalXdsInfo().ServiceNode, types.GetGlobalXdsInfo().ServiceCluster)
		err := sdsClient.sdsSubscriber.Start()
		if err != nil {
			log.DefaultLogger.Errorf("[sds] [sdsclient] sds subscriber start fail, %v", err)
			return nil
		}
		return sdsClient
	}
	return nil
}

// CloseSdsClientImpl used only mosn exit
func CloseSdsClient() {
	sdsClientLock.Lock()
	defer sdsClientLock.Unlock()
	if sdsClient != nil && sdsClient.sdsSubscriber != nil {
		sdsClient.sdsSubscriber.Stop()
		sdsClient.sdsSubscriber = nil
		sdsClient = nil
	}
}

func (client *SdsClientImpl) AddUpdateCallback(sdsConfig *auth.SdsSecretConfig, callback types.SdsUpdateCallbackFunc) error {
	client.updatedLock.Lock()
	defer client.updatedLock.Unlock()
	client.SdsConfigMap[sdsConfig.Name] = sdsConfig
	client.SdsCallbackMap[sdsConfig.Name] = callback
	client.sdsSubscriber.SendSdsRequest(sdsConfig.Name)
	return nil
}

// DeleteUpdateCallback
func (client *SdsClientImpl) DeleteUpdateCallback(sdsConfig *auth.SdsSecretConfig) error {
	client.updatedLock.Lock()
	defer client.updatedLock.Unlock()
	delete(client.SdsConfigMap, sdsConfig.Name)
	delete(client.SdsCallbackMap, sdsConfig.Name)
	return nil
}

// SetSecret invoked when sds subscriber get secret response
func (client *SdsClientImpl) SetSecret(name string, secret *auth.Secret) {
	client.updatedLock.Lock()
	defer client.updatedLock.Unlock()
	if fc, ok := client.SdsCallbackMap[name]; ok {
		log.DefaultLogger.Debugf("[xds] [sds client],set secret = %v", name)
		mosnSecret := types.SecretConvert(secret)
		fc(name, mosnSecret)
	}
}

// SetPostCallback
func SetSdsPostCallback(fc func()) {
	sdsPostCallback = fc
}

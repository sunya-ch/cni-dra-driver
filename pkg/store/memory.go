/*
Copyright 2025 The Kubernetes Authors.

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

package store

import (
	"sync"

	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

type Memory struct {
	mu           sync.RWMutex
	podResources map[types.UID][]*resourcev1beta1.ResourceClaim
}

func NewMemory() *Memory {
	return &Memory{
		podResources: map[types.UID][]*resourcev1beta1.ResourceClaim{},
	}
}

func (m *Memory) Add(podUID types.UID, claim *resourcev1beta1.ResourceClaim) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Avoid claims to be stored twice.
	storedClaims, exists := m.podResources[podUID]
	if exists {
		for _, storedClaim := range storedClaims {
			if claim.UID == storedClaim.UID { // already stored.
				return
			}
		}
	}

	m.podResources[podUID] = append(m.podResources[podUID], claim)
}

func (m *Memory) Get(podUID types.UID) []*resourcev1beta1.ResourceClaim {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.podResources[podUID]
}

func (m *Memory) Delete(podUID types.UID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.podResources, podUID)
}

# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REGISTRY ?= sigs.k8s.io/cni-dra-driver
VERSION ?= $(shell git describe --dirty --tags --always 2>/dev/null)

all: verify

.PHONY: verify
verify:
	hack/verify-all.sh

.PHONY: .build-image
build-image:
	docker build -t cni-dra-driver:$(VERSION) -f ./build/cni-dra-driver/Dockerfile .

.PHONY: push-image
push-image: build-image
	docker tag cni-dra-driver:$(VERSION) $(REGISTRY)/cni-dra-driver:$(VERSION)
	docker push $(REGISTRY)/cni-dra-driver:$(VERSION)

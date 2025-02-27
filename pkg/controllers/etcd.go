/*
Copyright © 2021 Microshift Contributors

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
package controllers

import (
	"fmt"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	etcd "go.etcd.io/etcd/embed"

	"github.com/openshift/microshift/pkg/config"
)

var (
	tlsCipherSuites = []string{
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
	}
)

const (
	etcdStartupTimeout = 10
)

func StartEtcd(c *config.MicroshiftConfig, ready chan bool) error {
	// based on https://github.com/openshift/cluster-etcd-operator/blob/master/bindata/bootkube/bootstrap-manifests/etcd-member-pod.yaml#L19
	cfg := etcd.NewConfig()
	cfg.ClusterState = "new"
	//cfg.ForceNewCluster = true //TODO
	cfg.Logger = "zap"
	cfg.Dir = c.DataDir + "/etcd/"
	cfg.APUrls = setURL([]string{c.HostIP}, ":2380")
	cfg.LPUrls = setURL([]string{c.HostIP}, ":2380")
	cfg.ACUrls = setURL([]string{c.HostIP}, ":2379")
	cfg.LCUrls = setURL([]string{"127.0.0.1", c.HostIP}, ":2379")
	cfg.ListenMetricsUrls = setURL([]string{"127.0.0.1"}, ":2381")

	cfg.Name = c.HostName
	cfg.InitialCluster = c.HostName + "=" + "https://" + c.HostIP + ":2380"

	cfg.CipherSuites = tlsCipherSuites
	cfg.ClientTLSInfo.CertFile = c.DataDir + "/certs/secrets/etcd-all-serving/etcd-serving.crt"
	cfg.ClientTLSInfo.KeyFile = c.DataDir + "/certs/secrets/etcd-all-serving/etcd-serving.key"
	cfg.ClientTLSInfo.TrustedCAFile = c.DataDir + "/certs/ca-bundle/ca-bundle.crt"
	cfg.ClientTLSInfo.ClientCertAuth = false
	cfg.ClientTLSInfo.InsecureSkipVerify = true //TODO after fix GenCert to generate client cert

	cfg.PeerTLSInfo.CertFile = c.DataDir + "/certs/secrets/etcd-all-peer/etcd-peer.crt"
	cfg.PeerTLSInfo.KeyFile = c.DataDir + "/certs/secrets/etcd-all-peer/etcd-peer.key"
	cfg.PeerTLSInfo.TrustedCAFile = c.DataDir + "/certs/ca-bundle/ca-bundle.crt"
	cfg.PeerTLSInfo.ClientCertAuth = false
	cfg.PeerTLSInfo.InsecureSkipVerify = true //TODO after fix GenCert to generate client cert

	e, err := etcd.StartEtcd(cfg)
	if err != nil {
		return fmt.Errorf("etcd failed to start: %v", err)
	}
	go func() {
		select {
		case <-e.Server.ReadyNotify():
			logrus.Info("Server is ready!")
			ready <- true
		case <-time.After(etcdStartupTimeout * time.Second):
			e.Server.Stop()
			logrus.Fatalf("etcd failed to start in %d seconds", etcdStartupTimeout)
		}
	}()
	return nil
}

func setURL(hostnames []string, port string) []url.URL {
	urls := make([]url.URL, len(hostnames))
	for i, name := range hostnames {
		u, err := url.Parse("https://" + name + port)
		if err != nil {
			return []url.URL{}
		}
		urls[i] = *u
	}
	return urls
}

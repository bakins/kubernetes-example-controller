package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const (
	group   = "akins.org"
	version = "v1alpha1"
)

// standard Kubernetes metadata
type metadata struct {
	Name            string            `json:"name,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	UID             string            `json:"uid,omitempty"`
	GenerateName    string            `json:"generateName,omitempty"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	OwnerReferences []ownerReference  `json:"ownerReferences,omitempty"`
}

type ownerReference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
	Controller bool   `json:"controller"`
}

// specification of a heap
type heapSpec struct {
	// Host for ingress. If empty, no ingress is created.
	Host string `json:"host,omitempty"`
	// Image to run. required.
	Image string `json:"image"`
	// Command to run. optional
	Command []string `json:"command"`
	// Replica count. defaults to 1
	Replicas uint32 `json:"replicas,omitempty"`
	// Port that is exposed in the container. If unset,
	// then no service or ingress is created.
	Port uint32 `json:"port,omitempty"`
}

type heap struct {
	Kind       string   `json:"kind"`
	APIVersion string   `json:"apiVersion"`
	Metadata   metadata `json:"metadata"`
	Spec       heapSpec `json:"spec"`
}

type heapList struct {
	Items []heap `json:"items"`
}

func main() {
	interval := flag.Duration("interval", time.Second*10, "loop interval")
	server := flag.String("server", "http://127.0.0.1:8001", "Kubernetes API server")
	flag.Parse()

	// run once at startup
	loop(*server)
	for range time.NewTicker(*interval).C {
		loop(*server)
	}
}

// silly  wrappers to mimic https://github.com/pkg/errors
func wrapError(err error, message string) error {
	if err != nil {
		return fmt.Errorf(message+" %v", err)
	}
	return errors.New(message)
}

func wrapErrorf(err error, format string, args ...interface{}) error {
	return wrapError(err, fmt.Sprintf(format, args...))
}

// run a single loop of the controller
func loop(server string) {
	heaps, err := listHeaps(server)
	if err != nil {
		log.Println("failed to list heaps", err)
		return
	}

	for _, h := range heaps {
		if h.Spec.Image == "" {
			sendEvent(server, &h, "missingImage", "no image specified")
			continue
		}

		if err := ensureDeployment(server, &h); err != nil {
			log.Println(err)
			sendEvent(server, &h, "deploymentFailed", err.Error())
			continue
		}

		if h.Spec.Port != 0 {
			if err := ensureService(server, &h); err != nil {
				log.Println(err)
				sendEvent(server, &h, "serviceFailed", err.Error())
				continue
			}

			if h.Spec.Host != "" {
				if err := ensureIngress(server, &h); err != nil {
					log.Println(err)
					sendEvent(server, &h, "ingressFailed", err.Error())
					continue
				}
			}
		}
	}
}

func createOwnerReferences(h *heap) []ownerReference {
	return []ownerReference{
		ownerReference{
			APIVersion: h.APIVersion,
			Kind:       h.Kind,
			Name:       h.Metadata.Name,
			UID:        h.Metadata.UID,
			Controller: true,
		},
	}
}

func listHeaps(server string) ([]heap, error) {
	u := fmt.Sprintf("%s/apis/%s/%s/heaps", server, group, version)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("listHeaps: unexpected status %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var list heapList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return list.Items, nil
}

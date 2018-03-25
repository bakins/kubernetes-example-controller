package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Kubernetes event - based on https://godoc.org/k8s.io/api/core/v1#Event
type event struct {
	Metadata       metadata        `json:"metadata"`
	Reason         string          `json:"reason,omitempty"`
	Message        string          `json:"message,omitempty"`
	InvolvedObject objectReference `json:"involvedObject"`
	Count          uint32
	FirstTimestamp time.Time
	LastTimestamp  time.Time
}

type objectReference struct {
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
	UID        string `json:"uid,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

func sendEvent(server string, h *heap, reason string, message string) {
	log.Println(h.Metadata.Namespace, h.Metadata.Name, reason, message)

	now := time.Now()

	e := event{
		Metadata: metadata{
			Namespace:    h.Metadata.Namespace,
			GenerateName: "heaps-" + h.Metadata.Name,
		},
		Reason:         reason,
		Message:        message,
		LastTimestamp:  now,
		FirstTimestamp: now,
		Count:          1,
		InvolvedObject: objectReference{
			Kind:       h.Kind,
			APIVersion: h.APIVersion,
			Name:       h.Metadata.Name,
			Namespace:  h.Metadata.Namespace,
			UID:        h.Metadata.UID,
		},
	}

	u := fmt.Sprintf("%s/api/v1/namespaces/%s/events",
		server, e.Metadata.Namespace)

	data, err := json.Marshal(e)

	if err != nil {
		log.Println("failed to marshal event", err)
		return
	}

	buf := bytes.NewBuffer(data)

	resp, err := http.Post(u, "application/json", buf)

	if err != nil {
		log.Println("failed to post event", err)
	}

	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != 201 {
		log.Println("sendEvent: unexpected status", resp.StatusCode)
	}
}

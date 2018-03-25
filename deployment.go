package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
)

type deployment struct {
	Metadata metadata       `json:"metadata"`
	Spec     deploymentSpec `json:"spec"`
}

type deploymentSpec struct {
	Replicas uint32          `json:"replicas"`
	Selector labelSelector   `json:"selector"`
	Template podTemplateSpec `json:"template"`
}

type podTemplateSpec struct {
	Metadata metadata `json:"metadata"`
	Spec     podSpec  `json:"spec,omitempty"`
}

type podSpec struct {
	Containers []container `json:"containers"`
}

type container struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command,omitempty"`
}

type labelSelector struct {
	MatchLabels map[string]string `json:"matchLabels"`
}

func ensureDeployment(server string, h *heap) error {
	replicas := h.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	labels := map[string]string{
		"heap": h.Metadata.Name,
	}

	d := deployment{
		Metadata: metadata{
			Name:            h.Metadata.Name,
			Namespace:       h.Metadata.Namespace,
			Labels:          h.Metadata.Labels,
			Annotations:     h.Metadata.Annotations,
			OwnerReferences: createOwnerReferences(h),
		},
		Spec: deploymentSpec{
			Replicas: replicas,
			Selector: labelSelector{
				MatchLabels: labels,
			},
			Template: podTemplateSpec{
				Metadata: metadata{
					Labels: labels,
				},
				Spec: podSpec{
					Containers: []container{
						container{
							Name:    h.Metadata.Name,
							Image:   h.Spec.Image,
							Command: h.Spec.Command,
						},
					},
				},
			},
		},
	}

	// check if it exists
	old, err := getDeployment(server, h.Metadata.Namespace, h.Metadata.Name)
	if err != nil {
		return wrapError(err, "failed to get deployment")
	}

	if old != nil {
		if deploymentsEqual(old, &d) {
			return nil
		}

		d.Metadata.ResourceVersion = old.Metadata.ResourceVersion
		// in a real controller, we would create and apply a patch if needed
		if err := deployDeployment(server, true, 200, &d); err != nil {
			return wrapError(err, "failed to update deployment")
		}
		sendEvent(server, h, "deploymentUpdated", "updated deployment")
		return nil
	}

	if err := deployDeployment(server, false, 201, &d); err != nil {
		return wrapError(err, "failed to create deployment")
	}

	sendEvent(server, h, "deploymentCreated", "created deployment")
	return nil
}

func deploymentsEqual(a *deployment, b *deployment) bool {
	return reflect.DeepEqual(a.Spec, b.Spec)
}

func getDeployment(server string, namespace string, name string) (*deployment, error) {
	u := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s",
		server, namespace, name)

	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close() //nolint: errcheck

	switch resp.StatusCode {
	case 404:
		return nil, nil
	case 200:
	default:
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var d deployment
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}

	return &d, nil
}

func deployDeployment(server string, update bool, expectedStatus int, d *deployment) error {
	var u string

	method := http.MethodPost
	if update {
		method = http.MethodPut
		u = fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s",
			server, d.Metadata.Namespace, d.Metadata.Name)
	} else {
		u = fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments",
			server, d.Metadata.Namespace)
	}

	data, err := json.Marshal(d)

	if err != nil {
		return wrapError(err, "failed to marshal deployment")
	}

	buf := bytes.NewBuffer(data)

	req, err := http.NewRequest(method, u, buf)
	if err != nil {
		return wrapError(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return wrapErrorf(err, "failed to %s deployment", method)
	}

	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != expectedStatus {
		return wrapErrorf(err, "unexpected status: %d", resp.StatusCode)
	}

	return nil
}

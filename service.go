package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
)

type service struct {
	Metadata metadata    `json:"metadata"`
	Spec     serviceSpec `json:"spec"`
}

type serviceSpec struct {
	Ports    []servicePort     `json:"ports"`
	Selector map[string]string `json:"selector"`
	Type     string            `json:"type"`
}

type servicePort struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Port       uint32 `json:"port"`
	TargetPort uint32 `json:"targetPort"`
}

func ensureService(server string, h *heap) error {
	s := service{
		Metadata: metadata{
			Name:            h.Metadata.Name,
			Namespace:       h.Metadata.Namespace,
			Labels:          h.Metadata.Labels,
			Annotations:     h.Metadata.Annotations,
			OwnerReferences: createOwnerReferences(h),
		},
		Spec: serviceSpec{
			Selector: map[string]string{
				"heap": h.Metadata.Name,
			},
			Type: "ClusterIP",
			Ports: []servicePort{
				servicePort{
					Name:       "heap",
					Port:       h.Spec.Port,
					TargetPort: h.Spec.Port,
					Protocol:   "TCP",
				},
			},
		},
	}

	// check if it exists
	old, err := getService(server, h.Metadata.Namespace, h.Metadata.Name)
	if err != nil {
		return err
	}

	if old != nil {
		if servicesEqual(old, &s) {
			return nil
		}

		s.Metadata.ResourceVersion = old.Metadata.ResourceVersion
		// in a real controller, we would create and apply a patch if needed
		if err := deployService(server, true, 200, &s); err != nil {
			return wrapError(err, "failed to update service")
		}
		sendEvent(server, h, "serviceUpdated", "updated service")
		return nil
	}

	if err := deployService(server, false, 201, &s); err != nil {
		return wrapError(err, "failed to create service")
	}

	sendEvent(server, h, "serviceCreated", "created service")

	return nil
}

func servicesEqual(a *service, b *service) bool {
	return reflect.DeepEqual(a.Spec, b.Spec)
}

func getService(server string, namespace string, name string) (*service, error) {
	u := fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s",
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

	var s service
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return &s, nil
}

func deployService(server string, update bool, expectedStatus int, s *service) error {
	var u string

	method := http.MethodPost
	if update {
		method = http.MethodPut
		u = fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s",
			server, s.Metadata.Namespace, s.Metadata.Name)
	} else {
		u = fmt.Sprintf("%s/api/v1/namespaces/%s/services",
			server, s.Metadata.Namespace)
	}

	data, err := json.Marshal(s)

	if err != nil {
		return wrapError(err, "failed to marshal service")
	}

	buf := bytes.NewBuffer(data)

	req, err := http.NewRequest(method, u, buf)
	if err != nil {
		return wrapError(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return wrapErrorf(err, "failed to %s service", method)
	}

	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != expectedStatus {
		return wrapErrorf(err, "unexpected status: %d", resp.StatusCode)
	}

	return nil
}

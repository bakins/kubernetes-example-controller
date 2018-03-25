package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
)

type ingress struct {
	Metadata metadata    `json:"metadata"`
	Spec     ingressSpec `json:"spec"`
}

type ingressSpec struct {
	Rules []ingressRule `json:"rules"`
}

type ingressRule struct {
	Host string          `json:"host"`
	HTTP httpIngressRule `json:"http"`
}

type httpIngressRule struct {
	Paths []httpIngressPath `json:"paths"`
}

type httpIngressPath struct {
	Path    string         `json:"path"`
	Backend ingressBackend `json:"backend"`
}

type ingressBackend struct {
	ServiceName string `json:"serviceName"`
	ServicePort string `json:"servicePort"`
}

func ensureIngress(server string, h *heap) error {
	i := ingress{
		Metadata: metadata{
			Name:            h.Metadata.Name,
			Namespace:       h.Metadata.Namespace,
			Labels:          h.Metadata.Labels,
			Annotations:     h.Metadata.Annotations,
			OwnerReferences: createOwnerReferences(h),
		},
		Spec: ingressSpec{
			Rules: []ingressRule{
				ingressRule{
					Host: h.Spec.Host,
					HTTP: httpIngressRule{
						Paths: []httpIngressPath{
							httpIngressPath{
								Path: "/",
								Backend: ingressBackend{
									ServiceName: h.Metadata.Name,
									ServicePort: "heap",
								},
							},
						},
					},
				},
			},
		},
	}

	// check if it exists
	old, err := getIngress(server, h.Metadata.Namespace, h.Metadata.Name)
	if err != nil {
		return err
	}

	if old != nil {
		if ingressesEqual(old, &i) {
			return nil
		}

		i.Metadata.ResourceVersion = old.Metadata.ResourceVersion

		// in a real controller, we would create and apply a patch if needed
		if err := deployIngress(server, true, 200, &i); err != nil {
			return wrapError(err, "failed to update ingress")
		}
		sendEvent(server, h, "ingressUpdated", "updated ingress")
		return nil
	}

	if err := deployIngress(server, false, 201, &i); err != nil {
		return wrapError(err, "failed to create ingress")
	}

	sendEvent(server, h, "ingressCreated", "created ingress")

	return nil
}

func ingressesEqual(a *ingress, b *ingress) bool {
	return reflect.DeepEqual(a.Spec, b.Spec)
}

func getIngress(server string, namespace string, name string) (*ingress, error) {
	u := fmt.Sprintf("%s/apis/extensions/v1beta1/namespaces/%s/ingresses/%s",
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

	var i ingress
	if err := json.Unmarshal(data, &i); err != nil {
		return nil, err
	}

	return &i, nil
}

func deployIngress(server string, update bool, expectedStatus int, i *ingress) error {
	var u string

	method := http.MethodPost
	if update {
		method = http.MethodPut
		u = fmt.Sprintf("%s/apis/extensions/v1beta1/namespaces/%s/ingresses/%s",
			server, i.Metadata.Namespace, i.Metadata.Name)
	} else {
		u = fmt.Sprintf("%s/apis/extensions/v1beta1/namespaces/%s/ingresses",
			server, i.Metadata.Namespace)
	}

	data, err := json.Marshal(i)

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
		return wrapErrorf(err, "failed to %s ingress", method)
	}

	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != expectedStatus {
		return wrapErrorf(err, "unexpected status: %d", resp.StatusCode)
	}

	return nil
}

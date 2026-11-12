package kubernetes

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/beacon-engine/pulse/internal/protocol"
)

func listServices(ctx context.Context, cs clientset.Interface) ([]protocol.ServiceInfo, error) {
	list, err := cs.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.ServiceInfo, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, mapService(&list.Items[i]))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func mapService(svc *corev1.Service) protocol.ServiceInfo {
	info := protocol.ServiceInfo{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Type:      string(svc.Spec.Type),
		ClusterIP: svc.Spec.ClusterIP,
		Ports:     make([]protocol.ServicePort, 0, len(svc.Spec.Ports)),
		Selector:  svc.Spec.Selector,
		Created:   timestamp(svc.CreationTimestamp),
	}
	if info.Type == "" {
		info.Type = string(corev1.ServiceTypeClusterIP)
	}
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ing.Hostname != "" {
			info.External = append(info.External, ing.Hostname)
		} else if ing.IP != "" {
			info.External = append(info.External, ing.IP)
		}
	}
	info.External = append(info.External, svc.Spec.ExternalIPs...)
	if svc.Spec.ExternalName != "" {
		info.External = append(info.External, svc.Spec.ExternalName)
	}
	for _, p := range svc.Spec.Ports {
		proto := string(p.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		info.Ports = append(info.Ports, protocol.ServicePort{
			Name: p.Name, Protocol: proto, Port: p.Port,
			TargetPort: p.TargetPort.String(), NodePort: p.NodePort,
		})
	}
	return info
}

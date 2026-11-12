package kubernetes

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/beacon-engine/pulse/internal/protocol"
)

func listNamespaces(ctx context.Context, cs *clientset.Clientset) ([]protocol.NamespaceInfo, error) {
	list, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]protocol.NamespaceInfo, 0, len(list.Items))
	for i := range list.Items {
		status := string(list.Items[i].Status.Phase)
		if status == "" {
			status = "Unknown"
		}
		result = append(result, protocol.NamespaceInfo{Name: list.Items[i].Name, Status: status})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

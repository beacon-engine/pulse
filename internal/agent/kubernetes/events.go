package kubernetes

import (
	"context"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// maxEvents caps the events returned: the newest, which are the ones worth reading.
const maxEvents = 300

func listEvents(ctx context.Context, cs clientset.Interface) ([]protocol.EventInfo, error) {
	list, err := cs.CoreV1().Events(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	type seen struct {
		info protocol.EventInfo
		at   time.Time
	}
	all := make([]seen, 0, len(list.Items))
	for i := range list.Items {
		e := &list.Items[i]
		at := eventTime(e)
		count := e.Count
		if e.Series != nil && e.Series.Count > count {
			count = e.Series.Count
		}
		if count == 0 {
			count = 1
		}
		source := e.Source.Component
		if source == "" {
			source = e.ReportingController
		}
		all = append(all, seen{at: at, info: protocol.EventInfo{
			Namespace: e.Namespace, Type: e.Type, Reason: e.Reason, Message: strings.TrimSpace(e.Message),
			ObjectKind: e.InvolvedObject.Kind, ObjectName: e.InvolvedObject.Name,
			Count: count, LastSeen: timestamp(metav1.NewTime(at)), Source: source,
		}})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].at.After(all[j].at) })
	if len(all) > maxEvents {
		all = all[:maxEvents]
	}
	result := make([]protocol.EventInfo, len(all))
	for i, e := range all {
		result[i] = e.info
	}
	return result, nil
}

// eventTime is when an event last happened; which field holds it depends on
// whether it was written through the old core/v1 or the newer events.k8s.io API.
func eventTime(e *corev1.Event) time.Time {
	switch {
	case e.Series != nil && !e.Series.LastObservedTime.IsZero():
		return e.Series.LastObservedTime.Time
	case !e.LastTimestamp.IsZero():
		return e.LastTimestamp.Time
	case !e.EventTime.IsZero():
		return e.EventTime.Time
	}
	return e.CreationTimestamp.Time
}

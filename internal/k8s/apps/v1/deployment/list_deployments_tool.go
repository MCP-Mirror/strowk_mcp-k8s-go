package deployment

import (
	"context"
	"sort"
	"time"

	"github.com/strowk/foxy-contexts/pkg/fxctx"
	"github.com/strowk/foxy-contexts/pkg/mcp"
	"github.com/strowk/foxy-contexts/pkg/toolinput"
	"github.com/strowk/mcp-k8s-go/internal/content"
	"github.com/strowk/mcp-k8s-go/internal/k8s"
	"github.com/strowk/mcp-k8s-go/internal/utils"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewListDeploymentsTool(pool k8s.ClientPool) fxctx.Tool {
	contextProperty := "context"
	namespaceProperty := "namespace"

	schema := toolinput.NewToolInputSchema(
		toolinput.WithString(contextProperty, "Name of the Kubernetes context to use, defaults to current context"),
		toolinput.WithString(namespaceProperty, "Namespace to list Deployments from, defaults to all namespaces"),
	)

	return fxctx.NewTool(
		&mcp.Tool{
			Name:        "list-k8s-deployments",
			Description: utils.Ptr("List Kubernetes Deployments with basic information"),
			InputSchema: schema.GetMcpToolInputSchema(),
		},
		func(args map[string]interface{}) *mcp.CallToolResult {
			input, err := schema.Validate(args)
			if err != nil {
				return utils.ErrResponse(err)
			}

			k8sCtx := input.StringOr(contextProperty, "")
			namespace := input.StringOr(namespaceProperty, "")

			clientset, err := pool.GetClientset(k8sCtx)
			if err != nil {
				return utils.ErrResponse(err)
			}

			var deployments *appsv1.DeploymentList
			if namespace == "" {
				// List Deployments from all namespaces
				deployments, err = clientset.AppsV1().Deployments(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
			} else {
				// List Deployments from specific namespace
				deployments, err = clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
			}

			if err != nil {
				return utils.ErrResponse(err)
			}

			sort.Slice(deployments.Items, func(i, j int) bool {
				// Sort by namespace, then by name
				if deployments.Items[i].Namespace == deployments.Items[j].Namespace {
					return deployments.Items[i].Name < deployments.Items[j].Name
				}
				return deployments.Items[i].Namespace < deployments.Items[j].Namespace
			})

			var contents []interface{} = make([]interface{}, len(deployments.Items))
			for i, deployment := range deployments.Items {
				// Calculate age
				age := time.Since(deployment.CreationTimestamp.Time)

				// Extract deployment status information
				desiredReplicas := int(*(deployment.Spec.Replicas))
				readyReplicas := deployment.Status.ReadyReplicas
				updatedReplicas := deployment.Status.UpdatedReplicas
				availableReplicas := deployment.Status.AvailableReplicas

				content, err := content.NewJsonContent(DeploymentInList{
					Name:              deployment.Name,
					Namespace:         deployment.Namespace,
					Age:               utils.FormatAge(age),
					DesiredReplicas:   desiredReplicas,
					ReadyReplicas:     int(readyReplicas),
					UpdatedReplicas:   int(updatedReplicas),
					AvailableReplicas: int(availableReplicas),
					CreatedAt:         deployment.CreationTimestamp.Format(time.RFC3339),
				})
				if err != nil {
					return utils.ErrResponse(err)
				}
				contents[i] = content
			}

			return &mcp.CallToolResult{
				Meta:    map[string]interface{}{},
				Content: contents,
				IsError: utils.Ptr(false),
			}
		},
	)
}

// DeploymentInList provides a structured representation of Deployment information
type DeploymentInList struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	Age               string `json:"age"`
	DesiredReplicas   int    `json:"desired_replicas"`
	ReadyReplicas     int    `json:"ready_replicas"`
	UpdatedReplicas   int    `json:"updated_replicas"`
	AvailableReplicas int    `json:"available_replicas"`
	CreatedAt         string `json:"created_at"`
}
package existing

import (
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
	// corev1 "k8s.io/api/core/v1"
	"testing"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/apimachinery/pkg/watch"
	// "k8s.io/client-go/tools/cache"
	// "github.com/davecgh/go-spew/spew"
)

type mockDiscoveryClient struct {
	mock.Mock
}

func (dc *mockDiscoveryClient) ServerGroups() (apiGroupList *metav1.APIGroupList, err error) {
	args := dc.Called()
	return args.Get(0).(*metav1.APIGroupList), args.Error(1)
}

func (dc *mockDiscoveryClient) ServerResources() ([]*metav1.APIResourceList, error) {
	args := dc.Called()
	return args.Get(0).([]*metav1.APIResourceList), args.Error(1)
}

var (
	// cut-down api list that only contains core and apps
	apiList = `typemeta:
kind: APIGroupList
apiversion: v1
groups:
- typemeta:
    kind: ""
    apiversion: ""
  name: ""
  versions:
  - groupversion: v1
    version: v1
  preferredversion:
    groupversion: v1
    version: v1
  serveraddressbyclientcidrs: []
- typemeta:
    kind: ""
    apiversion: ""
  name: apps
  versions:
  - groupversion: apps/v1
    version: v1
  - groupversion: apps/v1beta2
    version: v1beta2
  - groupversion: apps/v1beta1
    version: v1beta1
  preferredversion:
    groupversion: apps/v1
    version: v1
  serveraddressbyclientcidrs: []
`
	resourceList = ``
)

func TestCachingDiscoveredAPISandResources(t *testing.T) {
	return
}

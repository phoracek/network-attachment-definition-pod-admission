// TODO: rename to webhook.go
package admission

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	netclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
	clientset     *netclient.Clientset
)

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

type WebhookServer struct {
	Server *http.Server
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)

	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatalf("Error while obtaining cluster config: %v", err)
	}

	clientset, err = netclient.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Error building example clientset: %v", err)
	}
}

//
//

type defaultKubeClient struct {
	client kubernetes.Interface
}

func newDefaultKubeClient(clientset kubernetes.Interface) *defaultKubeClient {
	return &defaultKubeClient{clientset}
}

func (d *defaultKubeClient) GetRawWithPath(path string) ([]byte, error) {
	return d.client.ExtensionsV1beta1().RESTClient().Get().AbsPath(path).DoRaw()
}

func (d *defaultKubeClient) GetPod(namespace, name string) (*v1.Pod, error) {
	return d.client.Core().Pods(namespace).Get(name, metav1.GetOptions{})
}

func (d *defaultKubeClient) UpdatePodStatus(pod *v1.Pod) (*v1.Pod, error) {
	return d.client.Core().Pods(pod.Namespace).UpdateStatus(pod)
}

// Check whether the target resoured need to be mutated
func mutationRequired(ignoredList []string, pod *v1.Pod) bool {
	for _, namespace := range ignoredList {
		if pod.ObjectMeta.Namespace == namespace {
			glog.Infof("Skip mutation for %v for it' in special namespace: %v", pod.ObjectMeta.Name, pod.ObjectMeta.Namespace)
			return false
		}
	}

	netAn, netAnOk := pod.Annotations[NETWORKS_ANNOTATION]
	if !netAnOk {
		return false
	}
	b, err := parsePodNetworkAnnotation(netAn, "default")
	if err != nil {
		glog.Fatalf("Error while TODO: %v", err)
	}
	for _, network := range b {

		nad, err := clientset.K8sCniCncfIo().NetworkAttachmentDefinitions(network.Namespace).Get(network.Name, metav1.GetOptions{})
		if err != nil {
			glog.Fatalf("Error getting network attachment definition: %v", err)
		}
		glog.Infof("network attachment definition %s with config %q", nad.Name, nad.Spec.Config)
	}

	required := true

	glog.Infof("Mutation policy for %v/%v: required: %v", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name, required)
	return required
}

func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return patch
}

func createPatch(pod *corev1.Pod, annotations map[string]string) ([]byte, error) {
	var patch []patchOperation

	patch = append(patch, updateAnnotation(pod.Annotations, annotations)...)

	return json.Marshal(patch)
}

func (whsvr *WebhookServer) mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request
	var pod corev1.Pod

	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	if !mutationRequired(ignoredNamespaces, &pod) {
		glog.Infof("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := map[string]string{"petr": "was here"}
	patchBytes, err := createPatch(&pod, annotations)
	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (whsvr *WebhookServer) Serve(w http.ResponseWriter, r *http.Request) {
	var body []byte

	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.mutate(&ar)
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	glog.Infof("Ready to write response ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

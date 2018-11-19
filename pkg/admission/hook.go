// TODO: rename to webhook.go
package admission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"text/template"

	"github.com/golang/glog"
	netclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
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
	Config *Config
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

// Check whether the target resoured need to be mutated
func (whsvr *WebhookServer) mutationRequired(ignoredList []string, pod *v1.Pod) ([]byte, bool) {
	// TODO: combine all patches here
	var patches []interface{}

	for _, namespace := range ignoredList {
		if pod.ObjectMeta.Namespace == namespace {
			glog.Infof("Skip mutation for %v for it' in special namespace: %v", pod.ObjectMeta.Name, pod.ObjectMeta.Namespace)
			return nil, false
		}
	}

	netAn, netAnOk := pod.Annotations[NETWORKS_ANNOTATION]
	if !netAnOk {
		return nil, false
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

		var netConfig map[string]interface{}
		err = json.Unmarshal([]byte(nad.Spec.Config), &netConfig)
		if err != nil {
			glog.Errorf("Failed to unmashal network config %s: %v", nad.Spec.Config, err)
			return nil, false
		}

		netTypeRaw, netTypeFound := netConfig["type"]
		if !netTypeFound {
			glog.Errorf("Given network is missing type")
			return nil, false
		}

		netType := netTypeRaw.(string)
		glog.Infof("network attachment definition with type %s", netType)

		for _, rule := range whsvr.Config.Rules {
			if rule.Type == netType {
				glog.Infof("found a rule matching given type: %v", rule)

				t, err := template.New("").Parse(rule.Patch)
				if err != nil {
					glog.Errorf("Failed to parse template: %v", err)
					return nil, false
				}
				buff := new(bytes.Buffer)
				err = t.Execute(buff, map[string]interface{}{"Definition": nad, "Config": netConfig})
				if err != nil {
					glog.Errorf("Failed to execute template: %v", err)
					return nil, false
				}
				p := buff.Bytes()

				var subPatches []interface{}
				err = json.Unmarshal(p, &subPatches)
				if err != nil {
					glog.Errorf("Failed to unmashal patch %s: %v", string(p), err)
					return nil, false
				}

				patches = append(patches, subPatches...)
			}
		}
	}

	patch, err := json.Marshal(patches)

	glog.Infof("Mutation policy for %v/%v: required: %v", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name, true)
	return patch, true
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

	patch, required := whsvr.mutationRequired(ignoredNamespaces, &pod)

	if !required {
		glog.Infof("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	//
	glog.Infof("XXX combined patch: %v", string(patch))
	//

	glog.Infof("AdmissionResponse: patch=%v\n", string(patch))
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patch,
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

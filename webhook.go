package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
	"k8s.io/api/admission/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// Initialize serializer
var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

// WebhookServer is used to share data between main() and request handlers to avoid global variables
type WebhookServer struct {
	server    *http.Server // Webserver reference
	validator *Validator   // Validator object
}

// WhSvrParameters contains Webhook Server parameters passed from ARGV
type WhSvrParameters struct {
	port       int    // Webhook server port
	certFile   string // Path to the x509 certificate for https
	keyFile    string // Path to the x509 private key matching `CertFile`
	configFile string // Path to configuration file
}

// ConfigFile is used for deserializing config file
type ConfigFile struct {
	Kinds []Kind `yaml:"kinds"` // Array of kinds with rules to validate
}

// Kind is used for deserializing config file
type Kind struct {
	Name  string       `yaml:"name"`  // Name of the Kind to validate
	Rules []ConfigRule `yaml:"rules"` // Array of validation rules
}

// ConfigRule holds individual rule settings
type ConfigRule struct {
	Name     string `yaml:"name"`              // Rule name
	Jsonpath string `yaml:"jsonpath"`          // JSONPath query to extract value from validated object
	Regexp   string `yaml:"regexp,omitempty"`  // Regexp, which will be applied on extracted value
	Message  string `yaml:"message,omitempty"` // Error message returned to user when validation rejects object
}

// Stats, reads and parses config file
func (whsvr *WebhookServer) readConfig(configFile string) {
	// Stat config file
	if _, err := os.Stat(configFile); err == nil {
		// Read it
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			glog.Errorf("Failed to read config file: %s", err)
			return
		}

		// Parse it
		config := ConfigFile{}
		err = yaml.Unmarshal([]byte(data), &config)
		if err != nil {
			glog.Errorf("Failed to parse config file: %s", err)
			return
		}

		// Iterate over kinds and rules and add them to validator
		for _, kind := range config.Kinds {
			for _, rule := range kind.Rules {
				if err := whsvr.validator.AddRule(kind.Name, rule); err != nil {
					glog.Errorf("Parsing rule '%s' for kind '%s' failed: %s", rule.Name, kind.Name, err)
				}
			}
		}
	} else if os.IsNotExist(err) {
		glog.Errorf("Config file not found, no validation will be performed")
	} else {
		glog.Errorf("Failed to stat config file: %s", err)
	}
}

// NewWebhookServer creates new WebhookServer instance with initialized validator
func NewWebhookServer(port int, pair tls.Certificate) *WebhookServer {
	validator := NewValidator()
	return &WebhookServer{
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
		validator: validator,
	}
}

// This function validates that request is correct and executes Validator on deserialized object
func (whsvr *WebhookServer) validate(ar *v1beta1.AdmissionReview, response *v1beta1.AdmissionResponse) {
	req := ar.Request

	glog.Infof("AdmissionReview for Kind=%v, Name=%v UID=%v Operation=%v UserInfo=%v",
		req.Kind, req.Name, req.UID, req.Operation, req.UserInfo)

	// DELETE operation for example is currently not supported
	switch req.Operation {
	// Validate both CREATE and UPDATE operations, as UPDATE may bring invalid fields too
	case "CREATE", "UPDATE":
		// Currently, Kinds other than PodSecurityPolicy are not supported
		switch req.Kind.Kind {
		case "PodSecurityPolicy":
			// Parse received object, to make sure it's correct
			var object policyv1beta1.PodSecurityPolicy
			if err := json.Unmarshal(req.Object.Raw, &object); err != nil {
				glog.Errorf("Could not unmarshal raw object: %v", err)
				response.Result.Message = err.Error()
				return
			}

			// If object is correct, we can execute queries on it
			if err := whsvr.validator.Validate(string(req.UID), req.Kind.Kind, object); err != nil {
				response.Result.Message = err.Error()
				return
			}
		default:
			glog.Errorf("Kind=%v not supported", req.Kind.Kind)
			response.Result.Message = "Kind not supported"
			return
		}
	default:
		glog.Errorf("Operation=%s not supported", req.Operation)
		response.Result.Message = "Operation not supported"
		return
	}

	response.Allowed = true
}

// Serve method for webhook server
// Checks if request is correct, deserializes it and passes to validate function
func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	// Read request body
	var body []byte

	if r.Body != nil {
		var data []byte
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.Error("Failed to read request body")
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		body = data
	}

	// If request body is empty, return error
	if len(body) == 0 {
		glog.Error("Received request with empty body")
		http.Error(w, "Request body empty", http.StatusBadRequest)
		return
	}

	// Verify the content type is correct
	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		glog.Errorf("Invalid content type: %s. Expected `application/json`", contentType)
		http.Error(w, "Invalid content type. Expected `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	// Store response data
	admissionReview := v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			Result:  &metav1.Status{},
			Allowed: false,
		},
	}

	// Store deserialized request
	ar := v1beta1.AdmissionReview{}

	// Try to deserialize request
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode request body: %v", err)
		admissionReview.Response.Result.Message = err.Error()
	} else {
		admissionReview.Response.UID = ar.Request.UID
		// If deserialisation succeeded, validate the request
		whsvr.validate(&ar, admissionReview.Response)
	}

	// Encode response
	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
	}

	// And send it
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("Could not write response: %v", err), http.StatusInternalServerError)
	}
}

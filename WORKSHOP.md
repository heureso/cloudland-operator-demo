### Steps
1. Initialize operator SDK (Already done)
```operator-sdk init --domain heureso.com --repo github.com/cloudland-operator-demo/demo-operator```
2. Create API
```operator-sdk create api --group operator --version v1alpha1 --kind Minio --resource --controller```
Add the following code to "api/v1alpha1/minio_types.go"
```
	// User is the user needed to login to Minio UI
	// +kubebuilder:default=MINIO_USER
	// +kubebuilder:validation:Required
	User string `json:"user"`

	// Password is the user password needed to login to Minio UI
	// +kubebuilder:default=MINIO_PASSWORD
	// +kubebuilder:validation:Required
	Password string `json:"password"`

	// ForceRedeploy is any string, modifying this field instructs
	// the Operator to redeploy the Operand
	ForceRedeploy string `json:"forceRedeploy,omitempty"`
```
Run "make generate" to regenerate code after modifying this file
```make generate```
Run "make manifests" to generate a CRD that is based on the API we just defined and creates the RBAC-files
```make manifests```





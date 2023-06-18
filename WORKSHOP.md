### Steps
1. Initialize operator SDK (Already done)
```operator-sdk init --domain heureso.com --repo github.com/cloudland-operator-demo/demo-operator```
2. Create API
```operator-sdk create api --group operator --version v1alpha1 --kind Minio --resource --controller```


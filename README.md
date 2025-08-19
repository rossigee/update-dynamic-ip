# Dynamic IP updater

I have:

* A K8S cluster running in the cloud.
* A home office network, connected to the Internet via ordinary fibre connection with a dynamic IP address.
* Services running on the home office network that I need access to when mobile.

I want:

* The home office network to maintain an ExternalDNS service entry in the K8S cluster, containing the latest dynamic IP address.
* An annotation on the ExternalDNS service so that `external-dns` maintains an A record with my domain hosting provider.

## Requirements

* Go 1.25 or later (for development)
* Kubernetes cluster with appropriate RBAC permissions
* Docker (for containerized deployment)

## Setup cluster resources

First, create a namespace called 'dynamicips'. I'm using the plural term because there's no reason why this setup couldn't be used to track and the dynamic IPs of multiple branch offices and maintain them in an (internal or external) DNS system.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: dynamicips
```

Next, create the ExternalName Service that will be recieving the IP address and advertising it.

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    external-dns.alpha.kubernetes.io/hostname: myhouse1.dynamicip.acme.com
  labels:
    app: myhouse1
  name: myhouse1
  namespace: dynamicips
spec:
  externalName: 12.34.56.78
  selector:
    app: myhouse1
  sessionAffinity: None
  type: ExternalName
```

Then, create a ServiceAccount, Role and RoleBinding allowing the ServiceAccount to manage the Service entry.

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dynamicips
  namespace: dynamicips
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: dynamicips
  namespace: dynamicips
rules:
- apiGroups:
  - ""
  resources:
  - services
  verbs:
  - list
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: dynamicips
  namespace: dynamicips
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: dynamicips
subjects:
- kind: ServiceAccount
  name: dynamicips
  namespace: dynamicips
```

At this point the ServiceAccount will have a newly generated token. Now we need to obtain the token from the secret, base64-decode it to to obtain the JWT token, and take note of this token in order to configure the client side.

```bash
SA=$(kubectl -n dynamicips get serviceaccount dynamicips -ojsonpath="{.secrets[0].name}")
TOKEN=$(kubectl -n dynamicips get secret $SA -ojsonpath="{.data.token}" | base64 -d)
echo $TOKEN
```

You can use a [JWT token viewer tool](https://jwtpal.com) to get the main data from the token.

## Client configuration

Choose a suitable container host on the branch network that will have good uptime. Here, I am currently deploying this on a QNAP NAS, but I am considering moving it to one of my Raspberry Pis.

Create a K8S client configuration ('kubeconfig') file, containiner the K8S endpoint and JWT service token.

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: <get_this_from_your_own_kubeconfig_file>
    server: https://your-k8s-api-endpoint.acme.com
  name: production

users:
- name: "dynamicips"
  user:
    token: "<your_service_account_jwt_token_here>"

contexts:
- name: "production"
  context:
    user: "dynamicips"
    cluster: "production"
```

And a `docker-compose.yaml` or equivalent:

```yaml
version: '2.2'

services:
  dynamicips:
    image: rossigee/update-dynamic-ip
    init: true
    restart: always
    volumes:
    - /share/dockervols/dynamicips-config/config:/root/.kube/config
    command:
    - ./update-dynamic-ip
    - -namespace=dynamicips
    - -servicename=myhouse1

```

Of course, you also need to run it up.

```
docker-compose up -d
```

And check the logs for errors.

```
docker-compose logs -f
```

And check that it updates the IP address in the service when the dynamic IP changes.

```
kubectl -n dynamicips get service bankrut1 -ojsonpath="{.spec.externalName}"
```

And check that it gets updated in DNS. Check your DNS control panel for the authoritative check. Also, use `dig` to check it's propagation.

```
dig @1.1.1.1 A myhouse1.dynamicip.acme.com
```

There you have it. I hope you find this useful.

## Development

### Building from Source

This project requires Go 1.25 or later.

```bash
# Clone the repository
git clone https://github.com/rossigee/update-dynamic-ip.git
cd update-dynamic-ip

# Build the binary
make build

# Or use Go directly
go build -o update-dynamic-ip
```

### Available Make Targets

The project includes a Makefile with the following targets:

* `make build` - Build the binary
* `make test` - Run all tests
* `make lint` - Run golangci-lint v2.4.0 for code quality checks
* `make clean` - Clean build artifacts
* `make tidy` - Tidy Go modules
* `make deps` - Download dependencies
* `make check` - Run tests and linting
* `make all` - Complete build pipeline (clean, deps, build, test, lint)

### Testing

Run the test suite:

```bash
make test
```

Or use Go directly:

```bash
go test -v ./...
```

### Code Quality

This project uses golangci-lint v2.4.0 for code quality checks:

```bash
make lint
```

The linter will check for:
* Error handling
* Code simplicity and style
* Potential security issues
* Performance optimizations
* Code duplication
* Formatting consistency

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Make your changes
4. Run tests and linting (`make check`)
5. Commit your changes (`git commit -am 'Add some feature'`)
6. Push to the branch (`git push origin feature/your-feature`)
7. Create a Pull Request

Please ensure all tests pass and code passes linting before submitting a PR.

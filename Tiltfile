# Tilt >= v0.17.8 is required to handle escaping of colons in selector names and proper teardown of resources
load('ext://min_tilt_version', 'min_tilt_version')
min_tilt_version('0.17.8')

# We require at minimum CRD support, so need at least Kubernetes v1.16
load('ext://min_k8s_version', 'min_k8s_version')
min_k8s_version('1.16')

# Load the extension for live updating
load('ext://restart_process', 'docker_build_with_restart')

# Load the extension for helm_remote
load('ext://helm_remote', 'helm_remote')

# Load the registry helpers
load('deploy/tilt/dependencies/registry/Tiltfile', 'deploy_registry', 'image_resource')

# Load the password helpers
load('deploy/tilt/libraries/password/Tiltfile', 'generate_password')

# Load the secret helpers
load('deploy/tilt/libraries/secrets/Tiltfile', 'secret_exists', 'create_secret_if_not_exists')

config.define_string('boots_repo_path', args=False, usage='path to boots repository')
config.define_string('hegel_repo_path', args=False, usage='path to hegel repository')
cfg = config.parse()
hegel_repo_path = cfg.get('hegel_repo_path', '../hegel')
boots_repo_path = cfg.get('boots_repo_path', '../boots')

def load_from_repo_with_fallback(path, workload_name, fallback_yaml, fallback_deps=[]):
    repoTiltfile = os.path.join(path, 'Tiltfile')
    if os.path.exists(repoTiltfile):
        include(repoTiltfile)
    else:
        k8s_yaml(fallback_yaml)
        k8s_resource(
            workload=workload_name,
            resource_deps=fallback_deps
        )

# Load the multus configuration
load('deploy/tilt/dependencies/multus/Tiltfile', 'deploy_multus')
deploy_multus()

# Load the kubevirt helpers
load('deploy/tilt/dependencies/kubevirt/Tiltfile', 'deploy_kubevirt')
deploy_kubevirt()

# cert-manager
# Load the cert manager helpers
load('deploy/tilt/dependencies/cert-manager/TiltfileCertManager', 'cert_manager')
load('deploy/tilt/dependencies/cert-manager/TiltfileCertManagerIssuer', 'issuer', 'generate_certificate')
cert_manager(resource_deps=['multus'])
issuer(self_signed_ca_issuer_name='tink-ca',resource_deps=['wait-for-cert-manager-webhook'])

# # PostgreSQL
# Load the database helpers
load('deploy/tilt/dependencies/database/Tiltfile', 'deploy_database')
deploy_database(
    db_name='tinkerbell',
    db_user='tinkerbell',
    credentials_secret_name='tink-db-credentials',
    resource_deps=['multus']
)

# # Registry
registry_ip='172.30.200.3'
registry_mac='08:00:29:00:00:00'
registry_mask='/16'
registry_network_attachment='tink-dev'
deploy_registry(
    user='admin',
    ip=registry_ip,
    cert_issuer='tink-ca',
    cert_name='registry-server-certificate',
    pod_annotations='k8s\\.v1\\.cni\\.cncf\\.io/networks=[{"interface":"net1"\\,"mac":"'+registry_mac+'"\\,"ips":["'+registry_ip+registry_mask+'"]\\,"name":"'+registry_network_attachment+'"\\,"namespace":"default"}]',
    credentials_secret_name='tink-registry-credentials'
)

local_resource(
    'tink-worker-build',
    'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o deploy/kind/docker/tink-worker ./cmd/tink-worker',
    deps=[
        'go.mod',
        'go.sum',
        'cmd/tink-worker',
        'client',
        'protos'
    ]
)

local_resource(
    'tink-worker-docker-build',
    'docker build -t tink-worker:latest deploy/kind/docker/tink-worker/',
    resource_deps=[
        'tink-worker-build'
    ],
    deps=[
        'deploy/kind/docker/tink-worker'
    ],
)

image_resource(
    'tink-worker-docker-push',
    'tink-worker:latest',
    credentials_secret_name='tink-registry-credentials',
    resource_deps=[
        'tink-worker-docker-build',
        'docker-registry'
    ]
)

local_resource(
    'tink-server-build',
    'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/tink-server ./cmd/tink-server',
    deps=[
        'go.mod',
        'go.sum',
        'cmd/tink-server',
        'db',
        'grpc-server',
        'http-server',
        'metrics',
        'pkg',
        'protos'
    ],
)

docker_build_with_restart(
    'quay.io/tinkerbell/tink',
    '.',
    dockerfile_contents="""
FROM gcr.io/distroless/base:debug as debug
WORKDIR /
COPY build/tink-server /tink-server
ENTRYPOINT ["/tink-server"]
""",
    only=[
        './build/tink-server',
    ],
    target='debug',
    live_update=[
        sync('./build/tink-server', '/tink-server')
    ],
    entrypoint=[
        # Kubernetes deployment argments are ignored by
        # the restart process helper, so need to include
        # them here.
        '/tink-server',
        '--facility=onprem',
        '--ca-cert=/certs/ca.crt',
        '--tls-cert=/certs/tls.crt',
        '--tls-key=/certs/tls.key'
    ]
)

tink_ip = '172.30.200.4'

create_secret_if_not_exists('tink-credentials', USERNAME='admin', PASSWORD=generate_password())

tink_username = str(local("kubectl get secret tink-credentials -o jsonpath='{.data.USERNAME}' | base64 -d", quiet=True))
tink_password = str(local("kubectl get secret tink-credentials -o jsonpath='{.data.PASSWORD}' | base64 -d", quiet=True))

generate_certificate(
    issuer='tink-ca',
    name='tink-server-certificate',
    dnsNames=[
        'tink-server',
        'tink-server.default',
        'tink-server.default.svc',
        'tink-server.default.svc.cluster.local',
    ],
    ipAddresses=[tink_ip],
)

k8s_yaml('deploy/kind/tink-server.yaml')
k8s_resource(
    workload='tink-server',
    resource_deps=[
        'tink-ca-issuer',
        'db'
    ]
)

# TODO: Create tink-server secret for use in other components

# deploy hegel from locally checked out repo, falling back to static deployment
load_from_repo_with_fallback(hegel_repo_path, 'hegel', 'deploy/kind/hegel.yaml', ['tink-server','tink-mirror'])

k8s_yaml('deploy/kind/nginx.yaml')
k8s_resource(
    workload='tink-mirror',
    objects=[
        'webroot:persistentvolumeclaim',
    ],
    resource_deps=[
        'tink-server',
    ]
)

# deploy boots from locally checked out repo, falling back to static deployment
load_from_repo_with_fallback(boots_repo_path, 'boots', 'deploy/kind/boots.yaml', ['tink-server'])

# TODO: Better handling of OSIE pre-loading, potentially using the [file_sync_only](https://github.com/tilt-dev/tilt-extensions/tree/master/file_sync_only) Tilt extension
# TODO: preload appropriate images into registry (depends on templates)
# TODO: preload hardware data (depends on worker definitions)
# TODO: preload templates and workflows
# TODO: where does pb&j fit into this?
# TODO: should portal fit into this?

# TODO: rework Dockerfile so that it can be shared between release and Tilt, potentially can leverage [docker_build_sub](https://github.com/tilt-dev/tilt-extensions) Tilt extension
# TODO: ensure secret, password, and other generated items are only generated once
# TODO: factor out some of the dependencies into library files
# TODO: improve cleanup of resources, some things are being left behind making tilt up/tilt down/tilt up not work correctly
# TODO: see if calico/cilium as a cni would avoid having to use a custom cni plugin


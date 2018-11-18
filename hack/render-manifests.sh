#!/bin/bash -e

while [[ $# -gt 0 ]]; do
    case ${1} in
        --app)
            app="$2"
            shift
            ;;
        --namespace)
            namespace="$2"
            shift
            ;;
        --service)
            service="$2"
            shift
            ;;
        --deployment)
            deployment="$2"
            shift
            ;;
        --certs)
            certs="$2"
            shift
            ;;
        --webhook-url)
            webhook_url="$2"
            shift
            ;;
        --webhook-cfg)
            webhook_cfg="$2"
            shift
            ;;
        --service-account)
            service_account="$2"
            shift
            ;;
        --cluster-role)
            cluster_role="$2"
            shift
            ;;
        --cluster-role-binding)
            cluster_role_binding="$2"
            shift
            ;;
        --image)
            image="$2"
            shift
            ;;
        --image-pull-policy)
            image_pull_policy="$2"
            shift
            ;;
        --ca-bundle)
            ca_bundle="$2"
            shift
            ;;
    esac
    shift
done

[ -z ${app} ] && (echo 'please specify app name using --app'; exit 1)
[ -z ${namespace} ] && namespace=default
[ -z ${service} ] && service=${app}-svc
[ -z ${deployment} ] && deployment=${app}-deployment
[ -z ${secret} ] && secret=${app}-certs
[ -z ${webhook} ] && webhook=${app}-webhook-cfg
[ -z ${webhook_url} ] && webhook_url=${app}.nad-pod-webhook.example.com
[ -z ${webhook_cfg} ] && webhook_cfg=${app}-webhook-cfg
[ -z ${service_account} ] && service_account=${app}-acc
[ -z ${cluster_role} ] && cluster_role=${app}-cr
[ -z ${cluster_role_binding} ] && cluster_role_binding=${app}-crb
[ -z ${image} ] && image=phoracek/network-attachment-definition-pod-admission
[ -z ${image_pull_policy} ] && image_pull_policy=IfNotPresent
[ -z ${ca_bundle} ] && (echo 'please specify ca bundle using --ca-bundle'; exit 1)

mkdir -p _out
for template in templates/*.template; do
    sed \
        -e "s/\${APP}/${app}/g" \
        -e "s/\${NAMESPACE}/${namespace}/g" \
        -e "s/\${SERVICE}/${service}/g" \
        -e "s/\${DEPLOYMENT}/${deployment}/g" \
        -e "s/\${SECRET}/${secret}/g" \
        -e "s/\${WEBHOOK}/${webhook}/g" \
        -e "s/\${WEBHOOK_URL}/${webhook_url}/g" \
        -e "s/\${WEBHOOK_CFG}/${webhook_cfg}/g" \
        -e "s/\${SERVICE_ACCOUNT}/${service_account}/g" \
        -e "s/\${CLUSTER_ROLE}/${cluster_role}/g" \
        -e "s/\${CLUSTER_ROLE_BINDING}/${cluster_role_binding}/g" \
        -e "s/\${IMAGE}/${image}/g" \
        -e "s/\${IMAGE_PULL_POLICY}/${image_pull_policy}/g" \
        -e "s/\${CA_BUNDLE}/${ca_bundle}/g" \
        $template > _out/$(basename ${template%.*})
done

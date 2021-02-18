#!/usr/bin/env bash
K3D_REGISTRY_PORT=$(docker inspect $(docker ps -f name="registry" --format '{{.Names}}') | jq -r '.[] | .Config.Labels."k3s.registry.port.external"')
for image in $(docker-compose config | yq -r '.services[].image')
do
  # NEW_IMAGE=k3d-registry.localhost:${K3D_REGISTRY_PORT}/$image

  NEW_IMAGE=k3d-ephemeral.registry.localhost:${K3D_REGISTRY_PORT}/$image

  docker tag $image $NEW_IMAGE
  docker push $NEW_IMAGE
done
#KUBECONFIG=(k3d kubeconfig write) cat manifest.yml | envsubst | kubectl apply -f -
#k3d cluster delete

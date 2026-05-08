# Bootstrap — pasos manuales (una sola vez)

Estos pasos se corren UNA VEZ en un cluster nuevo.
Después de esto, todo lo demás se gestiona via ArgoCD.

## 1. Instalar ArgoCD

```bash
kubectl create namespace argocd
kubectl apply -n argocd --server-side \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl apply -n argocd --server-side \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.4.1/manifests/crds/applicationset-crd.yaml

# Esperar a que esté listo
kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=120s
```

## 2. Configurar acceso al repo Git (si es privado)

```bash
# HTTPS con token
argocd repo add https://github.com/leloir/leloir.git \
  --username <user> --password <token>

# O SSH
argocd repo add git@github.com:leloir/leloir.git \
  --ssh-private-key-path ~/.ssh/id_ed25519
```

## 3. Aplicar el root ApplicationSet (UNA VEZ)

```bash
kubectl apply -f deploy/root-appset.yaml
```

A partir de este momento, **cada carpeta en `deploy/apps/` es una Application**.
Para agregar un nuevo componente: crear la carpeta, commitear, push.
ArgoCD lo detecta y lo despliega automáticamente.

## Acceder a la UI

```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
# Usuario: admin
# Password: kubectl get secret argocd-initial-admin-secret -n argocd \
#             -o jsonpath="{.data.password}" | base64 -d
```

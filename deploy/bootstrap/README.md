# Bootstrap — pasos manuales (una sola vez por cluster)

Estos pasos se corren UNA VEZ en un cluster nuevo.
Después de esto, **todo lo demás se gestiona via ArgoCD** — nunca más `helm install` manual.

---

## Contexto del entorno (Leloir PoC)

El cluster k3s corre dentro de un container podman rootful en el HOST Fedora.
No se puede usar rootless podman porque no tiene permisos para crear cgroup directories.

**Levantar/bajar el cluster:**
```bash
sudo ./scripts/cluster-up.sh           # levanta k3s (puertos 443, 6443)
sudo ./scripts/cluster-up.sh --down    # baja y elimina el container
sudo ./scripts/cluster-up.sh --status  # estado actual
```

**Después de recrear el cluster**, siempre actualizar el kubeconfig:
```bash
sudo podman exec k3s-server cat /etc/rancher/k3s/k3s.yaml > ~/.kube/config
sudo chown $USER:$USER ~/.kube/config
```

---

## Paso 1 — Instalar ArgoCD

```bash
kubectl create namespace argocd

kubectl apply -n argocd --server-side \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# ApplicationSet CRD (necesario para ArgoCD v3.x)
kubectl apply -n argocd --server-side \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.4.1/manifests/crds/applicationset-crd.yaml

kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=120s
```

## Paso 2 — Aplicar el root ApplicationSet (UNA VEZ)

```bash
kubectl apply -f deploy/root-appset.yaml
```

**Eso es todo.** ArgoCD lee `deploy/apps/*/` y despliega cada carpeta automáticamente:
- `deploy/apps/ingress-nginx/` → NGINX Ingress Controller
- `deploy/apps/cert-manager/`  → cert-manager + ClusterIssuers Let's Encrypt
- `deploy/apps/prometheus/`    → Prometheus + Alertmanager + Grafana
- `deploy/apps/argocd-config/` → Ingress para argocd.leloir.cybercirujas.club
- `deploy/apps/sample-app/`    → App de prueba + PrometheusRules

## Paso 3 — Configurar acme-dns (una sola vez, para TLS sin puerto 80)

cert-manager usa DNS-01 via acme-dns.io. No requiere exponer el puerto 80.

```bash
# Registrar cuenta en acme-dns.io y crear el secret de Kubernetes
./scripts/acme-dns-register.sh
```

El script imprime:
1. Los registros CNAME a agregar en tu proveedor DNS
2. El comando `kubectl create secret` listo para copiar/pegar

Agregar los CNAMEs en tu proveedor DNS, luego ejecutar el comando del secret.

## Paso 4 — Agregar componentes futuros

```bash
mkdir deploy/apps/holmesgpt/
# crear Chart.yaml + values.yaml
git add . && git commit -m "add holmesgpt" && git push
# ArgoCD detecta la carpeta nueva y despliega solo
```

Cuando se agrega un subdomain nuevo:
1. Ejecutar `./scripts/acme-dns-register.sh --add <nuevo.dominio>` para obtener el CNAME
2. Agregar el CNAME en el proveedor DNS
3. Actualizar el secret existente (el script da las instrucciones)

---

## Acceder a la UI de ArgoCD

```bash
# Por port-forward (siempre funciona):
kubectl port-forward svc/argocd-server -n argocd 8080:443
# → https://localhost:8080  usuario: admin

# Password inicial:
kubectl get secret argocd-initial-admin-secret -n argocd \
  -o jsonpath="{.data.password}" | base64 -d

# Por dominio (requiere ingress-nginx up y DNS configurado):
# → https://argocd.leloir.cybercirujas.club
```

---

## Dominios configurados

| Servicio | URL |
|---|---|
| ArgoCD UI | https://argocd.leloir.cybercirujas.club |
| Grafana | https://grafana.leloir.cybercirujas.club |

DNS: registro A en tu proveedor → IP pública del router.
Router: forward 443 → IP local de la PC (HAProxy TCP passthrough / SNI).

---

## Troubleshooting

### SSH a router OpenBSD 6.x falla con "incorrect signature"

Fedora (OpenSSH 9.x) negocia por defecto `mlkem768x25519-sha256` (post-cuántico),
que es incompatible con OpenBSD 6.x. Forzar un KEX clásico:

```bash
ssh -p<SSH_PORT> -o KexAlgorithms=curve25519-sha256 <USER>@<ROUTER_IP>
```

Agregar a `~/.ssh/config` (archivo local, no commitear):

```
Host router-leloir
    HostName <ROUTER_IP>
    Port <SSH_PORT>
    User <USER>
    KexAlgorithms curve25519-sha256
```

> Copiar este bloque a tu `~/.ssh/config` con los valores reales. Ese archivo es local y no va a git.

---

## Repo Git

- Remoto actual: https://github.com/villadalmine/leloir (personal, PoC)
- Remoto futuro: https://github.com/leloir/leloir (org, cuando se cree)

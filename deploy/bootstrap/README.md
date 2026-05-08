# Bootstrap — pasos manuales (una sola vez por cluster)

Estos pasos se corren UNA VEZ en un cluster nuevo.
Después de esto, **todo lo demás se gestiona via ArgoCD** — nunca más `helm install` manual.

---

## Contexto del entorno (Leloir PoC)

El cluster k3s corre dentro de un container podman rootful en el HOST Fedora.
No se puede usar rootless podman porque no tiene permisos para crear cgroup directories.

**Levantar/bajar el cluster:**
```bash
sudo ./scripts/cluster-up.sh           # levanta k3s (puertos 80, 443, 6443)
sudo ./scripts/cluster-up.sh --down    # baja y elimina el container
sudo ./scripts/cluster-up.sh --status  # estado actual
```

**Después de recrear el cluster**, siempre actualizar el kubeconfig:
```bash
sudo podman exec k3s-server cat /etc/rancher/k3s/k3s.yaml > /home/dalmine/.kube/config
sudo chown dalmine:dalmine /home/dalmine/.kube/config
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
- `deploy/apps/prometheus/`    → Prometheus + Alertmanager + Grafana
- `deploy/apps/argocd-config/` → Ingress para argocd.leloir.cybercirujas.club
- `deploy/apps/sample-app/`    → App de prueba + PrometheusRules

## Paso 3 — Agregar componentes futuros

```bash
mkdir deploy/apps/holmesgpt/
# crear Chart.yaml + values.yaml
git add . && git commit -m "add holmesgpt" && git push
# ArgoCD detecta la carpeta nueva y despliega solo
```

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

DNS: registro A en Namecheap `leloir.cybercirujas.club` → IP pública del router.
HAProxy en el router: forward 80 y 443 → IP local de la PC.

---

## Troubleshooting

### SSH al router OpenBSD 6.x falla con "incorrect signature"

Fedora (OpenSSH 9.x) negocia por defecto `mlkem768x25519-sha256` (post-cuántico),
que es incompatible con OpenBSD 6.x. Forzar un KEX clásico:

```bash
ssh -p54222 -o KexAlgorithms=curve25519-sha256 dalmine@81.207.69.100
```

Para no tener que escribirlo siempre, agregar a `~/.ssh/config`:

```
Host router-leloir
    HostName 81.207.69.100
    Port 54222
    User dalmine
    KexAlgorithms curve25519-sha256
```

Luego conectar con: `ssh router-leloir`

---

## Repo Git

- Remoto actual: https://github.com/villadalmine/leloir (personal, PoC)
- Remoto futuro: https://github.com/leloir/leloir (org, cuando se cree)

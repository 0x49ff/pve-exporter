# Proxmox Node Exporter

```shell
docker run -it -p "8000:8000" \
  -e PVE_API_TOKEN="username@realm\!tokenname" \
  -e PVE_API_SECRET="" \
  -e PVE_PATH="/metrics" \
  -e PVE_ADDRESS=":8000" \
  -e PVE_ENDPOINT="" \
  --name proxmox-exporter \
  0x49f/pve-exporter:v1.0
```
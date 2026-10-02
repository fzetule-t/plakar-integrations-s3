
make build
rm ./s3_v*.ptar
plakar pkg create ./manifest.yaml v1.1.10
plakar pkg rm s3
plakar pkg add  -allow-unsigned ./s3_v1.1.10_linux_amd64.ptar
plakar pkg list
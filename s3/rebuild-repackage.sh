
make build
rm ./s3_v*.ptar
plakar pkg create ./manifest.yaml v1.2.5
plakar pkg rm s3
plakar pkg add  -allow-unsigned ./s3_v1.2.5_linux_amd64.ptar
plakar pkg list
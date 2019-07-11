apt update && apt install -y procps ca-certificates && apt clean
mkdir /conode_data
mkdir -p .local/share .config
ln -s /conode_data .local/share/conode
ln -s /conode_data .config/conode
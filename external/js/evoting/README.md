# Evoting

This package is split into two parts

* Frontend - made using [Vue](https://vuejs.org/)
* Server - written in node. Responsible for managing auth with Tequila.

# Setup

## Environmental Variables

The authentication server requires an environment variable

* `PRIVATE_KEY` - ed25519 private key to generate schnorr signature

## Common steps

```
cd frontend
npm install 
cd ../server
npm install
```

Edit `frontend/src/config.js` and `server/config.js` and update `masterKey` to the
one logged by cothority

## Dev

By default, the destination for `npm run build` is set to `../server/dist`. The
node server will manage the auth and *serve the static files* as well in a dev
environemnt (unlike production where nginx/apache does the latter). Therefore,
just run `npm run build` in the `frontend` directory. Then run the server by:

```
cd server
npm run dev
```

## Production

The production setup is to use nginx as a reverse proxy that would redirect all
requests to `/auth/(login|verify)` to the node process while all other requests
will be served by the Vue frontend.

You'd want to change `frontend/config/index.js`. Search for the `build` key and
change the `index` and `assetsRoot` keys

```
  ...
  build: {
    // Template for index.html
    index: /path/to/server/root/index.html

    // Paths
    assetsRoot: /path/to/server/root,

	...
```

Make sure node can write to the server root.

Running `cd frontend && npm run build` will now output the transpiled files to
the server root. Run the server in production mode by executing

```
cd server
npm run prod # TODO: replace with PM2
```

### Nginx configuration

Here's a sample nginx configuration

```
server {
        listen 80;
        server_name <hostname>;
        return 301 https://<hostname>$request_uri;
}

server {
        listen 443 ssl;
        server_name <hostname>;
        ssl_certificate <path/to/cert>;
        ssl_certificate_key <path/to/key>;

        location ~ ^/auth/(login|verify)$ {
				proxy_pass http://localhost:3000;
                proxy_http_version 1.1;
                proxy_set_header Upgrade $http_upgrade;
                proxy_set_header Connection 'upgrade';
                proxy_set_header X-Forwarded-Host $host:$server_port;
                proxy_set_header X-Forwarded-Server $host;
                proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                proxy_cache_bypass $http_upgrade;
        }

        location / {
                root <path/to/server/root>;
                index index.html;
        }
}
```

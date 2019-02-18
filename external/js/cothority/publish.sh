#!/bin/bash

npm run build
npm run bundle
npm run doc

cp package.json dist/.

# remove the private field of the package json
sed -i '/"private": true,/d' dist/package.json

cp README.md dist/.
cp -r doc dist/doc

if [ "$1" = "--link" ] || [ "$1" = "-l" ]; then
    # linking allow to use the package locally
    cd dist/
    npm link
else
    npm publish dist/ --dry-run
fi
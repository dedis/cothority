#!/bin/bash

npm run build

cp README.md dist/.
cp package.json dist/.

# remove the private field of the package json
sed -i '/"private": true,/d' dist/package.json

if [ "$1" = "--link" ] || [ "$1" = "-l" ]; then
    # linking allow to use the package locally
    cd dist/
    npm link
else
    npm run bundle
    npm run doc
    cp -r doc dist/doc

    npm publish dist/ --dry-run
fi
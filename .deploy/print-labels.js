// https://guyharwood.co.uk/2018/03/12/conditional-travis-builds-with-pull-request-labels/

// This script outputs each label's name found at the repo path and PR id given
// in argument. Each label is printed in a separate line. If the PR has labels
// "fix", "issues", "build-js-kyber", then the script will output:
//
// fix
// issues
// build-js-kyper
//
// One can then easily check if a label exist with 
// $ node print-labels.js dedis/cothority 2043 | grep -F -E "^build-js-cothority$"

'use strict'

const https = require('https')
const repoPath = process.argv[2]
const pullRequestId = process.argv[3]

if (!pullRequestId) {
  console.log('Missing argument: pull request id')
  process.exit(1)
}

const pullRequestUrl = `/repos/${repoPath}/pulls/${pullRequestId}`

const options = {
  hostname: 'api.github.com',
  path: pullRequestUrl,
  method: 'GET',
  headers: {
    'User-Agent': 'node/https'
  }
}

const parseResponse = (res) => {
  let labels
  try {
    labels = JSON.parse(res).labels
  } catch (err) {
    console.error(`error parsing labels for PR ${pullRequestId}`)
    console.error(err)
    process.exit(1)
  }

  console.log(labels.map(function(el){
      return el.name
  }).join("\n"))
}

https.get(options, (response) => {
  let data = ''

  response.on('data', (chunk) => {
    data += chunk
  })

  response.on('end', () => {
    parseResponse(data)
  })
}).on('error', (err) => {
  console.error('Error: ' + err.message)
})
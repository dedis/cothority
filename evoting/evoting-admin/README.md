# Evoting admin tool

This is the evoting master chain admin tool. It can be used to make a new master
chain:

```
$ ./evoting-admin -admins 0,1,2,3 -pin bf6d681a9e84e0046414b67d1bb3e6e4 -roster ../../conode/public.toml
I : (                               main.main:  83) - Master ID: [57 223 155 178 205 105 248 71 28 42 23 91 215 233 71 232 62 49 96 99 38 241 28 211 170 55 123 60 57 30 225 220]
I : (                               main.main:  86) - Master ID in hex: 39df9bb2cd69f8471c2a175bd7e947e83e31606326f11cd3aa377b3c391ee1dc
```

To see the current status of the master chain:

```
$ ./evoting-admin -show -roster leader.toml -id 39df9bb2cd69f8471c2a175bd7e947e83e31606326f11cd3aa377b3c391ee1dc 
 Admins: [0 1 2 3 4 5 6]
 Roster: [tls://localhost:7002 tls://localhost:7004 tls://localhost:7006]
    Key: 0d75f6903e7fbcb5e8623c942f707e4d36fbfbfdefdd7ae8b50633d0ed86a3a2
```

Note that -show requires both `-id` and `-roster` arguments.

## Editing an election

This tool allows you to dump the current state of an election into a JSON file, edit it, and load
the modified version back into the election.

Modifying an election is only allowed on an election with no votes. Once the first vote has been cast,
the election configuration remains unchanged.

In order to modify the election configuration, you must be logged into the web app, and you
must copy the signature cookie from the browser to the commandline. See the help message for the
`-sig` option.

Here's an example of dumping and loading an election:

```
$ evoting-admin -json -dumpelection -id 0a652443055f0f22f8fb49caba31a596cdb98e8fd229b8308a6ea495e1929ce2 -roster leader.toml > out.json
# edit the out.json file here, as desired
$ evoting-admin -sig signature=b38a1119060e273964f5a5c0784bf418ad6f2f60eec68e9f90f7040deab60b6111bd4b71a69d4156052f8126708d671bf35517c867d19a4cfca224eeaca5e406 \
  -user 289938 -id 0a652443055f0f22f8fb49caba31a596cdb98e8fd229b8308a6ea495e1929ce2 \
  -roster leader.toml -load out.json
```

Here is the JSON format of an election:

```
{
	"Name": {
		"de": "",
		"en": "Vote for the leader",
		"fr": "Scrutin pour le leader",
		"it": ""
	},
	"Creator": 289938,
	"Users": [
		200095,
		317736,
		279674
	],
	"Candidates": [
		123456
	],
	"MaxChoices": 1,
	"Subtitle": {
		"de": "",
		"en": "A subtitle here",
		"fr": "Une sous-titre ici",
		"it": ""
	},
	"MoreInfo": "",
	"Start": "2020-02-14T00:00:00+01:00",
	"End": "2020-02-24T23:59:00+01:00",
	"Theme": "epfl",
	"FooterText": "",
	"FooterTitle": "",
	"FooterPhone": "",
	"FooterEmail": ""
}
```

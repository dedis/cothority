//"use strict";
const chai = require("chai");
const cothority = require("../../lib");
const kyber = require("@dedis/kyber-js");
const helpers = require("../helpers.js");
const child_process = require("child_process");
const fs = require("fs");

const curve = new kyber.curve.edwards25519.Curve();
const proto = cothority.protobuf;
const skipchain = cothority.skipchain;
const misc = cothority.misc;
const net = cothority.net;
const expect = chai.expect;

describe.only("cisc client", () => {

  it("can retrieve updates from conodes", done => {

    after(function(){
      console.log("Cleanup...")
      killGolang();
    })
    runGolang()
      .then(data => {
        [roster, id] = data;

        const addr1 = roster.identities[0].websocketAddr;
        const socket = new net.Socket(addr1, "Skipchain");
        const requestStr = "DataUpdate";
        const responseStr = "DataUpdateReply";

        const request = { id: misc.hexToUint8Array(id) };

        console.log("Sending data", request)
        return socket.send(requestStr, responseStr, request);
      })
      .then(skipblocks => {
        console.log("Received data from the skipchain:", skipblocks);

        for(var i=0; i<skipblocks.update.length; i++){
          var skipblock = skipblocks.update[i];
          console.log(skipblock.data);
        }
        done();
      })
  });
});

var spawned_conodes;

function killGolang() {
  console.log("Killing all conodes...");

  spawned_conodes.kill();
  spawned_conodes.stdin.destroy();
  spawned_conodes.stdout.destroy();
  spawned_conodes.stderr.destroy();
  child_process.execSync("pkill go");
}

function runGolang() {
  const build_dir = process.cwd() + "/test/skipchain/build";
  const spawn = child_process.spawn;
  return new Promise(function(resolve, reject) {

    console.log("build_dir = " + build_dir);
    env = process.env;
    env['DEBUG_LVL'] = '2'
    env['DEBUG_COLOR'] = 'true'
    spawned_conodes = spawn("go", ["run", "main.go"], {
      cwd: build_dir,
      env: process.env,
      detached: true
    });
    spawned_conodes.unref();

    console.log("Spawned Conode PID: " + spawned_conodes.pid);
    spawned_conodes.on("error", err => {
      reject("Conode Error: " + err);
    });

    spawned_conodes.stdout.setEncoding("utf8");
    spawned_conodes.stdout.on("data", data => {
      if (!data.match(/OK/)) {
        reject("RECEIVED DATA (NOT OK) => " + data);
      }
      console.log("RECEIVED DATA (OK) => " + data);
      resolve(data);
    });
    
  }).then(data => {

    // read roster and genesis
    const group_file = build_dir + "/public.toml";
    const genesis_file = build_dir + "/genesis.txt";

    const groupToml = fs.readFileSync(group_file, "utf8");
    const genesisID = fs.readFileSync(genesis_file, "utf8");
    console.log("groupToml:  " + groupToml);
    const roster = cothority.Roster.fromTOML(groupToml);

    return Promise.resolve([roster, genesisID]);
  });
}
//"use strict";
const chai = require("chai");
const cothority = require("../../lib");
const kyber = require("@dedis/kyber-js");
const helpers = require("../helpers.js");
const child_process = require("child_process");
const fs = require("fs");

const expect = chai.expect;
const proto = cothority.protobuf;
const skipchain = cothority.skipchain;
const misc = cothority.misc;
const net = cothority.net;
const curve = new kyber.curve.edwards25519.Curve();


describe.only("skipchain client", () => {

  it("can retrieve updates from conodes", done => {

    after(function(){
      killGolang();
    })

    runGolang()
      .then(data => {
        [roster, id] = data;
        
        //const socket = new net.RosterSocket(roster, "Skipchain");
        var socket = new net.Socket(roster._identities[0].addr, "SkipChain");

        const requestStr = "GetUpdateChain";
        const responseStr = "GetUpdateChainReply";
        const request = { latestId: misc.hexToUint8Array(id) };

        return socket.send(requestStr, responseStr, request);
      })
      .then(data => {
        console.log("Received data from Server");
        console.log(data);
        done();
      })
  });
});

var spawned_conodes;

function runGolang() {  
  const build_dir = process.cwd() + "/test/cisc/build";
  const spawn = child_process.spawn;

  // start the process, and returns a promise
  return new Promise(function(resolve, reject) {
    console.log("build_dir = " + build_dir);
    spawned_conodes = spawn("go", ["run", "main.go"], {
      cwd: build_dir,
      env: process.env,
      detached: true
    });
    spawned_conodes.unref();
    console.log("Spawned Conode PID: " + spawned_conodes.pid);
    spawned_conodes.on("error", err => {
      reject("Error on Conode: " + err);
    });
    spawned_conodes.stdout.setEncoding("utf8");
    spawned_conodes.stdout.on("data", data => {
      if (!data.match(/OK/)) {
        reject("Data from Conode (NOT OK) => " + data);
      }
      console.log("Data from Conode (OK) => " + data);
      resolve(data);
    });
    spawned_conodes.on("exit", (code, signal) => {
      console.log("Conode exited.");
    });
  }).then(data => {
    // read roster and genesis
    const group_file = build_dir + "/public.toml";
    const genesis_file = build_dir + "/genesis.txt";

    const groupToml = fs.readFileSync(group_file, "utf8");
    const genesisID = fs.readFileSync(genesis_file, "utf8");
    console.log("groupToml:  " + groupToml);

    const roster = cothority.Roster.fromTOML(groupToml);
    return [roster, genesisID];
  });
}

function killGolang() {
  console.log("Killing spawned conodes");
  spawned_conodes.stdout.destroy();
  spawned_conodes.stderr.destroy();
  spawned_conodes.kill();
  child_process.execSync("pkill go");
}
